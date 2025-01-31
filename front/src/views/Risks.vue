<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <ApplicationFilter :applications="applications" @filter="setFilter" class="my-4" />

        <div class="legend mb-3">
            <div v-for="s in statuses" class="item">
                <div class="count" :class="s.color">{{ s.count }}</div>
                <div class="label">{{ s.name }}</div>
            </div>
            <v-checkbox
                label="Show dismissed"
                :value="showDismissed"
                @change="changeShowDismissed"
                class="font-weight-regular mt-0 pt-0 ml-2"
                style="margin-left: -4px"
                color="primary"
                hide-details
            />
        </div>

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="items"
            must-sort
            no-data-text="No risks found"
            ref="table"
            :headers="[
                { value: 'application_id', text: 'Application', sortable: false },
                { value: 'application_type', text: 'Application type', sortable: false },
                { value: 'severity', text: 'Risk category', sortable: false },
                { value: 'description', text: 'Description', sortable: false },
                { value: 'actions', text: '', sortable: false, align: 'end', width: '20px' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application_id="{ item }">
                <div class="application">
                    <div class="name">
                        <router-link :to="{ name: 'overview', params: { id: item.application_id }, query: $utils.contextQuery() }">
                            {{ $utils.appId(item.application_id).name }}
                        </router-link>
                    </div>
                </div>
            </template>

            <template #item.application_type="{ item }">
                <div v-if="item.application_type" class="d-flex align-center">
                    <img
                        v-if="item.application_type.icon"
                        :src="`${$coroot.base_path}static/img/tech-icons/${item.application_type.icon}.svg`"
                        onerror="this.style.display='none'"
                        height="16"
                        width="16"
                        class="icon"
                    />
                    <span class="type">{{ item.application_type.name }}</span>
                </div>
            </template>

            <template #item.severity="{ item }">
                <div class="risk">
                    <div class="status" :class="item.color" />
                    <span>{{ item.key.category }}</span>
                </div>
            </template>

            <template #item.description="{ item }">
                <div :class="{ 'grey--text': item.dismissal }">
                    <template v-if="item.exposure">
                        Publicly exposed database on
                        <template v-if="item.exposure.ips.length > 1">
                            {{ item.exposure.ips.length }} IPs
                            <v-menu offset-y tile>
                                <template #activator="{ on }">
                                    <span v-on="on" class="text-no-wrap ips"> {{ item.exposure.ips[0] }}</span>
                                </template>
                                <v-list dense>
                                    <v-list-item v-for="v in item.exposure.ips" style="font-size: 14px; min-height: 32px">
                                        <v-list-item-title>{{ v }}</v-list-item-title>
                                    </v-list-item>
                                </v-list>
                            </v-menu>
                        </template>
                        <span v-else>IP {{ item.exposure.ips[0] }}</span>
                        <template v-if="item.exposure.node_port_services">
                            through the NodePort {{ $pluralize('service', item.exposure.node_port_services.length) }}
                            <v-menu offset-y tile>
                                <template #activator="{ on }">
                                    <span v-on="on" class="text-no-wrap ips"> {{ item.exposure.node_port_services[0] }}</span>
                                </template>
                                <v-list dense>
                                    <v-list-item v-for="s in item.exposure.node_port_services" style="font-size: 14px; min-height: 32px">
                                        <v-list-item-title>{{ s }}</v-list-item-title>
                                    </v-list-item>
                                </v-list>
                            </v-menu>
                        </template>
                        <template v-else-if="item.exposure.load_balancer_services">
                            through the LoadBalancer {{ $pluralize('service', item.exposure.load_balancer_services.length) }}
                            <v-menu offset-y tile>
                                <template #activator="{ on }">
                                    <span v-on="on" class="text-no-wrap ips"> {{ item.exposure.load_balancer_services[0] }}</span>
                                </template>
                                <v-list dense>
                                    <v-list-item v-for="s in item.exposure.load_balancer_services" style="font-size: 14px; min-height: 32px">
                                        <v-list-item-title>{{ s }}</v-list-item-title>
                                    </v-list-item>
                                </v-list>
                            </v-menu>
                        </template>
                        <template v-else> {{ $pluralize('port', item.exposure.ports.length) }} {{ item.exposure.ports.join(', ') }} </template>
                    </template>
                </div>
                <div v-if="item.dismissal" class="caption">
                    Dismissed by {{ item.dismissal.by }} ({{ $format.date(item.dismissal.timestamp * 1000, '{YYYY}-{MM}-{DD} {HH}:{mm}:{ss}') }}) as
                    "{{ item.dismissal.reason }}"
                </div>
            </template>

            <template #item.actions="{ item }">
                <v-menu offset-y>
                    <template v-slot:activator="{ attrs, on }">
                        <v-btn icon x-small class="ml-1" v-bind="attrs" v-on="on">
                            <v-icon small>mdi-dots-vertical</v-icon>
                        </v-btn>
                    </template>

                    <v-list dense>
                        <template v-if="!item.dismissal">
                            <v-list-item @click="post('dismiss', item.key, item.application_id, 'tolerable for this project')">
                                <v-icon small class="mr-1">mdi-bell-off-outline</v-icon> Dismiss: tolerable for this project
                            </v-list-item>
                            <v-list-item
                                v-if="item.exposure"
                                @click="post('dismiss', item.key, item.application_id, 'controlled by network policies')"
                            >
                                <v-icon small class="mr-1">mdi-security-network</v-icon> Dismiss: controlled by network policies
                            </v-list-item>
                        </template>
                        <v-list-item v-else @click="post('mark_as_active', item.key, item.application_id)">
                            <v-icon small class="mr-1">mdi-bell-outline</v-icon> Mark as Active
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import ApplicationFilter from '../components/ApplicationFilter.vue';

const statuses = {
    critical: { name: 'Critical', color: 'red lighten-1' },
    warning: { name: 'Warning', color: 'orange lighten-1' },
    ok: { name: 'Dismissed', color: 'grey lighten-1' },
};

export default {
    components: { ApplicationFilter },

    data() {
        return {
            loading: false,
            error: '',
            risks: [],
            showDismissed: false,
            filter: new Set(),
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
        this.showDismissed = this.$route.query.show_dismissed === '1';
    },

    watch: {
        items() {
            if (this.items.some((i) => i.severity === 'ok') && !this.showDismissed) {
                this.showDismissed = true;
            }
        },
    },

    computed: {
        applications() {
            if (!this.risks) {
                return [];
            }
            const applications = {};

            this.risks.forEach((v) => {
                applications[v.application_id] = v.application_category;
            });
            return Object.keys(applications).map((id) => ({ id, category: applications[id] }));
        },
        items() {
            if (!this.risks) {
                return [];
            }
            let filtered = this.risks.filter((v) => this.filter.has(v.application_id));
            const shd = this.$route.query.show_dismissed;
            if (shd === '0') {
                filtered = filtered.filter((i) => i.severity !== 'ok');
            }
            if (shd === undefined) {
                const undismissed = filtered.filter((i) => i.severity !== 'ok');
                if (undismissed.length) {
                    filtered = undismissed;
                }
            }
            return filtered.map((i) => {
                return {
                    ...i,
                    color: statuses[i.severity].color,
                };
            });
        },
        statuses() {
            return Object.keys(statuses).map((s) => {
                return {
                    ...statuses[s],
                    count: this.risks.filter((i) => i.severity === s).length,
                };
            });
        },
    },

    methods: {
        get() {
            this.loading = true;
            const query = this.$route.query.query || '';
            this.$api.getOverview('risks', query, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.risks = data.risks || [];
            });
        },
        post(action, key, app_id, reason) {
            this.loading = true;
            this.error = '';
            this.$api.risks(app_id, { key, action, reason }, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
            });
        },
        changeShowDismissed() {
            this.showDismissed = !this.showDismissed;
            this.$router.push({ query: { ...this.$route.query, show_dismissed: this.showDismissed ? '1' : '0' } }).catch((err) => err);
        },
        setFilter(filter) {
            this.filter = filter;
        },
    },
};
</script>

<style scoped>
.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(th) {
    white-space: nowrap;
}
.table .application {
    display: flex;
    gap: 4px;
}
.table .application .name {
    max-width: 30ch;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.risk {
    gap: 4px;
    display: flex;
}
.risk .status {
    height: 20px;
    width: 4px;
}
.ips {
    border-bottom: 1px dashed darkgray;
    cursor: pointer;
}
.icon {
    margin-right: 4px;
    opacity: 80%;
}
.type {
    opacity: 60%;
    white-space: nowrap;
}
.legend {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
    font-weight: 500;
    font-size: 14px;
}
.legend .item {
    display: flex;
    gap: 4px;
}
.legend .count {
    padding: 0 4px;
    border-radius: 2px;
    height: 18px;
    line-height: 18px;
    color: rgba(255, 255, 255, 0.8);
}
.legend .label {
    opacity: 60%;
}
</style>
