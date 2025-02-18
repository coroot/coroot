<template>
    <div>
        <p>
            This integration enables Coroot to discover RDS and ElastiCache instances and collect their telemetry data. It requires permissions to
            describe RDS and ElastiCache instances, read their logs and read Enhanced Monitoring data from CloudWatch.
        </p>

        <p>
            <b>Step #1</b>: create an
            <a
                href="https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_create-console.html#access_policies_create-json-editor"
                target="_blank"
            >
                IAM policy
            </a>
            with the <a @click="policyDialog = true">following permissions</a>.
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

        <p>
            <b>Step #2</b>: create an
            <a href="https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html#id_users_create_console" target="_blank">IAM user</a>
            with programmatic access, attach the policy to it and use AccessKeyID/SecretAccessKey in the form below.
        </p>

        <v-form v-if="form" v-model="valid" ref="form">
            <div class="subtitle-1 mt-3">Region</div>
            <div class="caption">Coroot only discovers RDS and ElastiCache instances within the specified region, e.g. <var>us-west-1</var></div>
            <v-text-field v-model="form.region" :rules="[$validators.notEmpty]" outlined dense hide-details single-line clearable />

            <div class="subtitle-1 mt-3">Access Key ID</div>
            <v-text-field v-model="form.access_key_id" :rules="[$validators.notEmpty]" outlined dense hide-details single-line />

            <div class="subtitle-1 mt-3">Secret Access Key</div>
            <v-text-field v-model="form.secret_access_key" :rules="[$validators.notEmpty]" outlined dense hide-details single-line type="password" />

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

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3">
                {{ error }}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text class="mt-3">
                {{ message }}
            </v-alert>
            <div class="mt-3">
                <v-btn v-if="saved.region && !form.region" block color="error" @click="del" :loading="loading">Delete</v-btn>
                <v-btn v-else block color="primary" @click="save" :disabled="!valid" :loading="loading">Save</v-btn>
            </div>
        </v-form>

        <h2 class="text-h6 mt-10 mb-3">Discovery status</h2>
        <v-alert v-if="form && !form.region" color="primary" outlined text> Not configured </v-alert>
        <v-alert v-else-if="errors.length" color="error" outlined text class="pb-2">
            <div v-for="e in errors" class="mb-2">• {{ e }}</div>
        </v-alert>
        <v-alert v-else-if="!error" color="success" outlined text> OK </v-alert>
        <v-alert v-else outlined text> Unknown </v-alert>

        <h2 class="text-h6 mt-10 mb-3">Discovered instances</h2>
        <v-data-table
            :items="instances"
            sort-by="application_id"
            must-sort
            dense
            class="instances"
            mobile-breakpoint="0"
            :items-per-page="20"
            no-data-text="No instances found"
            :headers="[
                { value: 'application_id', text: 'Application', align: 'start' },
                { value: 'name', text: 'Instance', align: 'start' },
                { value: 'status', text: 'Status', align: 'start' },
                { value: 'engine', text: 'Engine', align: 'start' },
                { value: 'engine_version', text: 'Version', align: 'start' },
                { value: 'instance_type', text: 'Instance type', align: 'start' },
                { value: 'availability_zone', text: 'AZ', align: 'start' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application_id="{ item }">
                <router-link :to="{ name: 'overview', params: { view: 'applications', id: item.application_id } }" class="text-no-wrap">
                    {{ $utils.appId(item.application_id).name }}
                </router-link>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import Code from '../components/Code.vue';

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
    components: { Code },

    data() {
        return {
            form: null,
            valid: false,
            loading: false,
            error: '',
            message: '',
            rds_tag_filters: '',
            elasticache_tag_filters: '',
            saved: null,
            policyDialog: false,
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
                this.saved = JSON.parse(JSON.stringify(this.form));
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
