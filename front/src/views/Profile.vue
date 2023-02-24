<template>
<div>
    <v-card outlined class="my-4 pa-4">
        <div>
            <Led :status="view.status" />
            <template v-if="view.message">
                <span v-html="view.message" />
                <span v-if="view.status !== 'warning'">
                    (<a @click="configure = true">configure</a>)
                </span>
            </template>
            <span v-else>Loading...</span>
            <v-progress-circular v-if="loading" indeterminate size="16" width="2" color="green" />
        </div>
        <v-select v-if="view.profiles" :value="profile" :items="profiles" @change="setProfile" outlined hide-details dense :menu-props="{offsetY: true}" class="mt-4" />
        <div class="grey--text mt-3">
            <v-icon size="20" style="vertical-align: baseline">mdi-lightbulb-on-outline</v-icon>
            Select a chart area to zoom in or compare with the previous period
        </div>
    </v-card>

    <Chart v-if="view.chart" :chart="view.chart" class="my-5" :selection="selection" @select="setSelection" />

    <div ref="flamegraph"></div>

    <v-dialog v-model="configure" max-width="800">
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                Link "{{ $api.appId(appId).name }}" with a Pyroscope application
                <v-spacer />
                <v-btn icon @click="configure = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div class="subtitle-1">Choose a corresponding Pyroscope application:</div>
            <v-select v-model="form.application" :items="applications" outlined dense hide-details :menu-props="{offsetY: true}" clearable />

            <div class="grey--text my-4">
                To configure an application to send profiles to Pyroscope follow the <a href="">documentation</a>.
            </div>

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="my-3">
                {{error}}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text class="my-3">
                {{message}}
            </v-alert>
            <v-btn block color="primary" @click="save" :loading="saving" :disabled="!changed" class="mt-5">Save</v-btn>
        </v-card>
    </v-dialog>

</div>
</template>

<script>
import '@pyroscope/flamegraph/dist/index.css';
import {FlamegraphRenderer} from '@pyroscope/flamegraph';
import Chart from "@/components/Chart.vue";
import Led from "@/components/Led.vue";
import React from "react";
import ReactDom from "react-dom/client";

export default {
    props: {
        appId: String,
    },

    components: {Chart, Led},

    data() {
        return {
            loading: false,

            view: {},

            configure: false,
            form: {
                application: null,
            },
            saved: '',
            saving: false,
            error: '',
            message: '',

            flamegraph: null,
        }
    },

    computed: {
        profiles() {
            return (this.view.profiles || []).map(p => ({
                text: p.type + (p.name ? ` (${p.name})` : ''),
                value: {type: p.type, name: p.name},
            }));
        },
        profile() {
            const p = this.getProfile();
            return {type: p.type, name: p.name};
        },
        selection() {
            const p = this.getProfile();
            return {mode: p.mode, from: p.from, to: p.to};
        },
        applications() {
            return (this.view.applications || []).map(a => a.name);
        },
        changed() {
            return !!this.form && this.saved !== JSON.stringify(this.form);
        },
    },

    mounted() {
        this.flamegraph = ReactDom.createRoot(this.$refs.flamegraph);
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    beforeDestroy() {
        this.$router.replace({query: {...this.$route.query, profile: undefined}}).catch(err => err);
        this.flamegraph.unmount();
    },

    watch: {
        'view.profile'(v) {
            if (!v) {
                this.flamegraph.render(null);
                return;
            }
            this.flamegraph.render(React.createElement(FlamegraphRenderer, {
                profile: v,
                onlyDisplay: 'flamegraph',
                colorMode: 'light',
            }));
        }
    },

    methods: {
        setSelection(e) {
            this.setProfile({mode: e.selection.mode, from: e.selection.from, to: e.selection.to}, e.ctx);
        },
        getProfile() {
            const parts = (this.$route.query.profile || '').split(':');
            return {
                type: parts[0] || '',
                name: parts[1] || '',
                mode: parts[2] || 'diff',
                from: Number(parts[3]) || 0,
                to: Number(parts[4]) || 0,
            };
        },
        setProfile(p, ctx) {
            p = {...this.getProfile(), ...p};
            const profile = `${p.type}:${p.name}:${p.mode}:${p.from}:${p.to}`;
            if (this.$route.query.profile !== profile) {
                this.$router.push({query: {...this.$route.query, ...ctx, profile}}).catch(err => err);
            }
        },
        get() {
            this.loading = true;
            this.error = '';
            // this.view.profile = null;
            // this.view.chart = null;
            this.$api.getProfile(this.appId, this.$route.query.profile, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.view = data;
                const application = (this.view.applications || []).find((a) => a.linked);
                this.form.application = application ? application.name : null;
                this.saved = JSON.stringify(this.form);
                const profile = (this.view.profiles || []).find((p) => p.selected);
                if (profile) {
                    this.setProfile({type: profile.type, name: profile.name});
                }
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveProfileSettings(this.appId, this.form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                    this.configure = false;
                }, 1000);
                this.get();
            });
        },
    },
};
</script>

<style scoped>
* >>> [role=heading] {
    color: var(--ps-neutral-2);
}
* >>> [data-testid=flamegraph-view] {
    margin-right: 0;
}
</style>
