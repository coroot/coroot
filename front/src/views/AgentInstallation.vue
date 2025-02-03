<template>
    <v-dialog v-model="dialog">
        <template #activator="{ on, attrs }">
            <v-btn :color="color" :outlined="outlined" :small="small" v-bind="attrs" v-on="on">
                <slot></slot>
            </v-btn>
        </template>
        <v-card class="pa-5">
            <div class="d-flex align-center text-h5 mb-4">
                Node agent installation
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <p>
                <a href="https://github.com/coroot/coroot-node-agent" target="_blank">Coroot-node-agent</a> gathers metrics, traces, logs, and
                profiles, and sends them to Coroot. To ingest telemetry data, the agent must have the address of the Coroot instance and the
                capability to establish TCP connections with it.
            </p>

            <v-form v-model="valid">
                <div class="subtitle-1">Coroot URL:</div>
                <v-text-field
                    v-model="coroot_url"
                    :rules="[$validators.notEmpty, $validators.isUrl]"
                    placeholder="http://coroot:8080"
                    outlined
                    dense
                />
                <div class="subtitle-1">
                    API Key (can be managed in the
                    <router-link :to="{ name: 'project_settings' }"><span @click="dialog = false">project settings</span></router-link
                    >):
                </div>
                <v-select
                    v-model="api_key"
                    :rules="[$validators.notEmpty]"
                    :items="api_keys === 'permission denied' ? [] : api_keys.map((k) => ({ value: k.key, text: `${k.key} (${k.description})` }))"
                    outlined
                    dense
                    :menu-props="{ offsetY: true }"
                    :no-data-text="api_keys === 'permission denied' ? 'Only project Admins can access API keys.' : 'No keys available'"
                />
            </v-form>

            <v-tabs v-model="tab" height="40" slider-size="2" class="mb-4">
                <v-tab><v-icon class="mr-1">mdi-memory</v-icon>Linux node (Systemd)</v-tab>
                <v-tab><v-icon class="mr-1">mdi-docker</v-icon>Docker</v-tab>
                <v-tab><v-icon class="mr-1">mdi-kubernetes</v-icon>Kubernetes</v-tab>
            </v-tabs>
            <v-tabs-items v-model="tab">
                <v-tab-item transition="none">
                    <p>
                        This script downloads the latest version of the agent and installs it as a Systemd service. Additionally, it generates an
                        uninstall script.
                    </p>
                    <Code :disabled="!valid">
                        <pre>
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT={{ coroot_url || '&lt;COROOT_URL_HERE&gt;' }} \
  API_KEY={{ api_key || '&lt;API_KEY_HERE&gt;' }} \
  SCRAPE_INTERVAL={{ scrape_interval }} \
  sh -
                        </pre>
                    </Code>
                    <p>You can read the agent log using the <var>journalctl</var> command:</p>
                    <Code>
                        <pre>
sudo journalctl -u coroot-node-agent
                        </pre>
                    </Code>
                    <p>To uninstall the agent run the command below:</p>
                    <Code>
                        <pre>
/usr/bin/coroot-node-agent-uninstall.sh
                        </pre>
                    </Code>
                </v-tab-item>

                <v-tab-item transition="none">
                    <Code :disabled="!valid">
                        <pre>
docker run --detach --name coroot-node-agent \
  --pull=always \
  --privileged --pid host \
  -v /sys/kernel/debug:/sys/kernel/debug:rw \
  -v /sys/fs/cgroup:/host/sys/fs/cgroup:ro \
  ghcr.io/coroot/coroot-node-agent:latest \
  --cgroupfs-root=/host/sys/fs/cgroup \
  --collector-endpoint={{ coroot_url || '&lt;COROOT_URL_HERE&gt;' }} \
  --api-key={{ api_key }} \
  --scrape-interval={{ scrape_interval }}
                        </pre>
                    </Code>
                    <p>To read the agent log:</p>
                    <Code>
                        <pre>
docker logs coroot-node-agent
                        </pre>
                    </Code>
                    <p>To uninstall the agent run the command below:</p>
                    <Code>
                        <pre>
docker rm -f coroot-node-agent
                        </pre>
                    </Code>
                </v-tab-item>
                <v-tab-item transition="none">
                    <p>Add the Coroot helm chart repo:</p>

                    <Code>
                        <pre>
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
                        </pre>
                    </Code>

                    <p>Next, install the Coroot Operator:</p>

                    <Code>
                        <pre>
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
                        </pre>
                    </Code>

                    <p>Install Coroot's agents (node-agent and cluster-agent):</p>

                    <Code :disabled="!valid">
                        <pre>
helm install -n coroot coroot coroot/{{ helm_chart }} --set "apiKey={{ api_key }},agentsOnly.corootURL={{ coroot_url || '&lt;COROOT_URL_HERE&gt;' }}"
                        </pre>
                    </Code>
                </v-tab-item>
            </v-tabs-items>
        </v-card>
    </v-dialog>
</template>

<script>
import Code from '../components/Code.vue';

export default {
    props: {
        color: String,
        outlined: Boolean,
        small: Boolean,
    },

    components: { Code },

    data() {
        const local = ['127.0.0.1', 'localhost'].some((v) => location.origin.includes(v));
        return {
            error: '',
            dialog: false,
            tab: null,
            coroot_url: !local ? location.origin : '',
            helm_chart: window.coroot.edition === 'Enterprise' ? 'coroot-ee' : 'coroot-ce',
            api_keys: [],
            api_key: '',
            scrape_interval: '15s',
            valid: false,
        };
    },

    watch: {
        dialog() {
            this.dialog && this.get();
        },
    },

    methods: {
        get() {
            this.$api.getProject(this.$route.params.projectId, (data, error) => {
                if (error) {
                    this.error = error;
                    return;
                }
                this.api_keys = data.api_keys || [];
                if (data.refresh_interval) {
                    this.scrape_interval = data.refresh_interval / 1000 + 's';
                }
            });
        },
    },
};
</script>

<style scoped></style>
