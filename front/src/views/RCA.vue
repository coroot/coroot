<template>
    <div>
        <v-progress-linear v-if="loading" indeterminate color="green" class="mt-5" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-alert v-if="rca === 'not implemented'" color="info" outlined text class="mt-5">
            Automated Root Cause Analysis is available only in Coroot Enterprise.
            <a href="https://coroot.com/contact/" target="_blank" class="font-weight-bold">Contact us</a> for a free trial.
        </v-alert>

        <div v-else-if="rca" class="mt-5">
            <div v-if="rca.causes && rca.causes.length > 0">
                <div class="mt-5 mb-3 text-h6">Possible causes</div>

                <v-simple-table dense>
                    <thead>
                        <tr>
                            <th>Issue</th>
                            <th>Reasons</th>
                            <th>Applications</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="c in rca.causes">
                            <td class="text-no-wrap">
                                <div>
                                    <v-icon color="error" small class="mr-1">mdi-alert-circle</v-icon>
                                    <span v-html="c.summary" />
                                </div>
                            </td>
                            <td class="text-no-wrap">
                                <ul>
                                    <li v-for="d in c.details">
                                        <span v-html="d" />
                                    </li>
                                </ul>
                            </td>
                            <td>
                                <div class="d-flex flex-wrap">
                                    <template v-for="(s, i) in c.affected_services">
                                        <router-link :to="{ name: 'application', params: { id: s }, query: $utils.contextQuery() }">
                                            {{ $utils.appId(s).name }}
                                        </router-link>
                                        <span v-if="i + 1 < c.affected_services.length" class="mr-1">,</span>
                                    </template>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
            </div>

            <div v-if="tree.length">
                <div class="mt-5 mb-3 text-h6">Detailed RCA report</div>

                <div>
                    <RcaItem v-for="h in tree" :key="h.id" :hyp="h" :split="70" />
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import RcaItem from '@/components/RcaItem.vue';
import { palette } from '@/utils/colors';

export default {
    computed: {
        tree() {
            if (!this.rca.hypotheses || !this.rca.hypotheses.length) {
                return [];
            }
            const f = (s, parent) => {
                const h = {
                    id: s.id,
                    name: s.name,
                    children: [],
                    level: parent.level + 1,
                    service: s.service,
                    color: palette.hash2(s.service),
                    timeseries: s.timeseries,
                    possible_cause: s.possible_cause,
                    widgets: s.widgets,
                    disable_reason: s.disable_reason,
                    log_pattern: s.log_pattern,
                };
                parent.children.push(h);
                this.rca.hypotheses
                    .filter((s) => s.parent_id === h.id)
                    .forEach((s) => {
                        f(s, h);
                    });
            };
            const root = { level: -1, children: [] };
            f(this.rca.hypotheses[0], root);
            return root.children;
        },
    },
    props: {
        appId: String,
    },

    components: { RcaItem },

    data() {
        return {
            rca: null,
            loading: false,
            error: '',
        };
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getRCA(this.appId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.rca = data;
            });
        },
    },
};
</script>

<style scoped></style>
