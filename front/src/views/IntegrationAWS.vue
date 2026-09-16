<template>
    <div>
        <p style="max-width: 800px">
            This integration enables Coroot to discover RDS and ElastiCache instances and collect their telemetry data: instance status, OS metrics
            from Enhanced Monitoring, and database logs. The recommended setup lets the cluster-agent use the IAM role of its pod (EKS Pod Identity or
            IRSA) and declares the integration and the database credentials in the Coroot custom resource, see the
            <a href="https://docs.coroot.com/configuration/aws" target="_blank">AWS integration</a> page and the
            <a href="https://docs.coroot.com/guides/aws-eks-rds" target="_blank">Monitoring Amazon RDS and ElastiCache from EKS</a> guide.
        </p>
        <p style="max-width: 800px">
            Alternatively, the integration can be configured here with the keys of an IAM user:
            <a @click="showForm = !showForm">{{ showForm ? 'hide the settings' : 'show the settings' }}</a>
        </p>
        <v-dialog v-model="policyDialog" max-width="800">
            <v-card class="pa-5">
                <div class="text-h6 d-flex mb-5">
                    MonitoringReadOnlyAccess role
                    <v-spacer />
                    <v-btn icon @click="policyDialog = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <Code>
                    <pre>
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "rds:DescribeDBInstances",
                "rds:DescribeDBLogFiles",
                "rds:DownloadDBLogFilePortion",
                "rds:ListTagsForResource",
                "elasticache:DescribeCacheClusters",
                "elasticache:ListTagsForResource"
            ],
            "Resource": [
                "*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "logs:GetLogEvents"
            ],
            "Resource": [
                "arn:aws:logs:*:*:log-group:RDSOSMetrics:log-stream:*"
            ]
        }
    ]
}
                    </pre>
                </Code>
            </v-card>
        </v-dialog>
        <v-form v-if="form && showForm" v-model="valid" ref="form" style="max-width: 800px">
            <p>
                <b>Step #1</b>: create an
                <a
                    href="https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_create-console.html#access_policies_create-json-editor"
                    target="_blank"
                >
                    IAM policy
                </a>
                with the <a @click="policyDialog = true">following permissions</a>. <b>Step #2</b>: create an
                <a href="https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html#id_users_create_console" target="_blank">IAM user</a>
                with programmatic access, attach the policy to it and use its AccessKeyID/SecretAccessKey.
            </p>
            <div class="subtitle-1 mt-3">Region</div>
            <div class="caption">
                Coroot only discovers RDS and ElastiCache instances within the specified region, e.g. <var>us-west-1</var>. Leave it empty to use the
                region the cluster-agent runs in.
            </div>
            <v-text-field v-model="form.region" outlined dense hide-details single-line clearable />

            <div class="subtitle-1 mt-3">Access Key ID</div>
            <v-text-field v-model="form.access_key_id" outlined dense hide-details single-line />

            <div class="subtitle-1 mt-3">Secret Access Key</div>
            <v-text-field v-model="form.secret_access_key" outlined dense hide-details single-line type="password" />

            <div class="subtitle-1 mt-3">RDS tag filters</div>
            <div class="caption">
                You can limit the discovery of RDS instances by filtering them based on their tags.
                <br />
                Specify tag_name=tag_value pairs, <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a> are
                supported for the value part, e.g. <var>team=qa,env=staging*</var>.
            </div>
            <v-text-field v-model="rds_tag_filters" outlined dense hide-details single-line />
            <div class="subtitle-1 mt-3">ElastiCache tag filters</div>
            <div class="caption">
                You can limit the discovery of ElastiCache instances by filtering them based on their tags.
                <br />
                Specify tag_name=tag_value pairs, <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a> are
                supported for the value part, e.g. <var>team=qa,env=staging*</var>.
            </div>
            <v-text-field v-model="elasticache_tag_filters" outlined dense hide-details single-line />

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3" style="max-width: 800px">
                {{ error }}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text class="mt-3">
                {{ message }}
            </v-alert>
            <div class="mt-3 d-flex" style="gap: 8px">
                <v-btn color="primary" class="flex-grow-1" @click="save" :disabled="!valid" :loading="loading">Save</v-btn>
                <v-btn v-if="configured" color="error" outlined @click="del" :loading="loading">Delete</v-btn>
            </div>
        </v-form>
        <CloudDiscovery provider="AWS" :configured="configured" :detected="detected" :errors="errors" :instances="instances" :error="error" />
    </div>
</template>

<script>
import Code from '../components/Code.vue';
import CloudDiscovery from '../components/CloudDiscovery.vue';

function map2str(m) {
    return Object.entries(m || {})
        .map(([k, v]) => `${k}=${v}`)
        .join(', ');
}

function str2map(s) {
    const res = {};
    s.split(',').forEach((f) => {
        const [k, v] = f.split('=');
        if (k && v && k.trim() && v.trim()) {
            res[k.trim()] = v.trim();
        }
    });
    return res;
}

export default {
    components: { Code, CloudDiscovery },

    data() {
        return {
            form: null,
            valid: false,
            loading: false,
            error: '',
            message: '',
            rds_tag_filters: '',
            elasticache_tag_filters: '',
            configured: false,
            detected: false,
            policyDialog: false,
            showForm: false,
            errors: [],
            instances: [],
        };
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('aws', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data.form;
                this.rds_tag_filters = map2str(this.form.rds_tag_filters);
                this.elasticache_tag_filters = map2str(this.form.elasticache_tag_filters);
                this.configured = !!data.view.configured;
                this.detected = !!data.view.detected;
                this.showForm = this.showForm || this.configured; // settings saved here stay visible
                this.errors = data.view.errors || [];
                this.instances = data.view.instances || [];
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.form.rds_tag_filters = str2map(this.rds_tag_filters);
            this.form.elasticache_tag_filters = str2map(this.elasticache_tag_filters);
            const form = JSON.parse(JSON.stringify(this.form));
            this.$api.saveIntegrations('aws', 'save', form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated. The changes will take effect in a minute or two.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
                this.get();
            });
        },
        del() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('aws', 'del', null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
            });
        },
    },
};
</script>

<style scoped></style>
