<template>
    <div class="dashboard">
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <template v-else>
            <div class="d-flex mb-3">
                <h1 class="text-h5">{{ dashboard.name }}</h1>
                <v-spacer />
                <div v-if="edit" class="d-flex gap-1">
                    <v-btn color="primary" plain @click="panel = {}">Add panel</v-btn>
                    <v-btn color="primary" @click="save">Save</v-btn>
                    <v-btn color="primary" outlined @click="cancel">Cancel</v-btn>
                </div>
                <div v-else class="d-flex gap-1">
                    <v-btn icon @click="$events.emit('refresh')"><v-icon>mdi-refresh</v-icon></v-btn>
                    <v-btn icon @click="edit = true"><v-icon>mdi-pencil-outline</v-icon></v-btn>
                </div>
            </div>

            <div v-if="!groups.length" class="text-center pt-10">
                <div class="grey--text mb-3">No panels are configured yet.</div>
                <v-btn
                    color="primary"
                    @click="
                        () => {
                            edit = true;
                            panel = {};
                        }
                    "
                >
                    <v-icon>mdi-plus</v-icon>
                    Add panel
                </v-btn>
            </div>

            <div v-for="(g, gi) in groups" class="group mb-2">
                <div class="d-flex align-center header">
                    <h2 v-if="g.name" class="text-h6">{{ g.name }}</h2>
                    <v-btn icon @click="g.collapsed = !g.collapsed">
                        <v-icon v-if="g.collapsed">mdi-chevron-right</v-icon>
                        <v-icon v-else>mdi-chevron-down</v-icon>
                    </v-btn>
                    <v-spacer />
                    <div v-if="edit" class="d-flex">
                        <v-btn small icon @click="group = { action: 'edit', id: gi, name: g.name }">
                            <v-icon small>mdi-pencil-outline</v-icon>
                        </v-btn>
                        <v-btn small icon @click="group = { action: 'delete', id: gi, name: g.name }">
                            <v-icon small>mdi-trash-can-outline</v-icon>
                        </v-btn>
                        <v-btn small icon @click="moveGroup(gi, 'down')" :disabled="gi >= groups.length - 1">
                            <v-icon small>mdi-arrow-down</v-icon>
                        </v-btn>
                        <v-btn small icon @click="moveGroup(gi, 'up')" :disabled="gi <= 0">
                            <v-icon small>mdi-arrow-up</v-icon>
                        </v-btn>
                    </div>
                </div>
                <div class="grid-stack" :class="{ 'd-none': g.collapsed }">
                    <div v-for="(p, pi) in g.panels" class="grid-stack-item" :gs-x="p.box.x" :gs-y="p.box.y" :gs-w="p.box.w" :gs-h="p.box.h">
                        <Panel
                            :config="p"
                            :buttons="edit"
                            @edit="panel = { config: p, group: g.name, gi, pi }"
                            @remove="delPanel(gi, pi)"
                            class="panel"
                        />
                    </div>
                </div>
            </div>
            <PanelForm v-if="panel" v-model="panel" :groups="groups.map((g) => g.name)" @add="addPanel" @edit="editPanel" />
            <GroupForm v-if="group" v-model="group" @edit="editGroup" @delete="delGroup" />
        </template>
    </div>
</template>

<script>
import { GridStack } from 'gridstack';
import 'gridstack/dist/gridstack.min.css';
import Panel from '@/views/dashboards/Panel.vue';
import PanelForm from '@/views/dashboards/PanelForm.vue';
import GroupForm from '@/views/dashboards/GroupForm.vue';

const gsOptions = {
    animate: false,
    staticGrid: true,
    cellHeight: 80,
    alwaysShowResizeHandle: true,
    handle: '.drag',
    margin: 0,
    acceptWidgets: true,
    minRow: 1,
};

export default {
    props: {
        id: String,
    },

    components: { Panel, PanelForm, GroupForm },

    data() {
        return {
            loading: false,
            error: '',
            dashboard: {
                name: '',
                config: {},
            },
            saved: '',
            edit: false,
            panel: null,
            group: null,
        };
    },

    created() {
        this.grids = [];
    },

    mounted() {
        this.get();
    },

    watch: {
        edit(v) {
            this.grids.forEach((g) => {
                g.setStatic(!v);
            });
        },
        dashboard: {
            handler() {
                this.redraw();
            },
            deep: true,
        },
    },

    computed: {
        groups() {
            return this.dashboard.config.groups || [];
        },
    },

    methods: {
        addPanel(p) {
            if (!this.dashboard.config.groups) {
                this.$set(this.dashboard.config, 'groups', []);
            }
            let gi = this.groups.findIndex((g) => g.name === p.group);
            if (gi === -1) {
                gi = this.groups.push({ name: p.group, panels: [], collapsed: false }) - 1;
            }
            p.config.box = { x: 0, y: 0, w: 6, h: 3 };
            this.groups[gi].panels.push(p.config);
        },
        editPanel(p) {
            let gi = this.groups.findIndex((g) => g.name === p.group);
            if (gi === -1) {
                gi = this.groups.push({ name: p.group, panels: [], collapsed: false }) - 1;
            }
            let pi = p.pi;
            if (this.groups[p.gi].name !== p.group) {
                this.groups[p.gi].panels.splice(p.pi, 1);
                pi = this.groups[gi].panels.push({}) - 1;
            }
            this.$set(this.groups[gi].panels, pi, p.config);
        },
        delPanel(gi, pi) {
            this.groups[gi].panels.splice(pi, 1);
        },
        moveGroup(gi, direction) {
            const groups = this.dashboard.config.groups;
            const gii = direction === 'down' ? gi + 1 : gi - 1;
            const g = groups[gi];
            this.$set(groups, gi, groups[gii]);
            this.$set(groups, gii, g);
        },
        editGroup(gi, name) {
            this.groups[gi].name = name;
        },
        delGroup(gi) {
            this.groups.splice(gi, 1);
        },
        redraw() {
            if (!this.grids) {
                return;
            }
            const groups = this.groups.splice(0, this.groups.length);
            this.grids.forEach((g) => {
                g.destroy(false);
            });
            this.grids = null;
            this.$nextTick(() => {
                this.$set(this.dashboard.config, 'groups', groups);
                this.$nextTick(() => {
                    this.draw();
                });
            });
        },
        draw() {
            if (!this.groups.length) {
                this.grids = [];
                return;
            }
            this.grids = GridStack.initAll({ ...gsOptions, staticGrid: !this.edit });
            this.groups?.forEach((g, gi) => {
                this.grids[gi].id_ = gi;
                this.grids[gi].group_ = g;
                const items = this.grids[gi].getGridItems();
                g.panels?.forEach((p, pi) => {
                    const node = items[pi].gridstackNode;
                    node.id_ = pi;
                    node.panel_ = p;
                });
            });
            this.grids.forEach((g) => {
                g.on('added', (e, nodes) => {
                    nodes.forEach((node) => {
                        const { x, y, w, h } = node;
                        Object.assign(node.panel_.box, { x, y, w, h });
                        node.id_ = this.groups[node.grid.id_].panels.push(node.panel_) - 1;
                    });
                });
                g.on('removed', (e, nodes) => {
                    nodes.forEach((node) => {
                        const { x, y, w, h } = node;
                        Object.assign(node.panel_.box, { x, y, w, h });
                        this.groups[node.grid.id_].panels.splice(node.id_, 1);
                    });
                });
                g.on('change', (e, nodes) => {
                    nodes.forEach((node) => {
                        const { x, y, w, h } = node;
                        Object.assign(node.panel_.box, { x, y, w, h });
                    });
                });
            });
        },
        cancel() {
            this.edit = false;
            this.dashboard = JSON.parse(this.saved);
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.dashboards(this.id, null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.dashboard = data || {};
                this.saved = JSON.stringify(this.dashboard);
                this.redraw();
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.$api.dashboards(this.id, this.dashboard, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
                this.edit = false;
            });
        },
    },
};
</script>

<style scoped>
.grid-stack {
    margin: 0 -4px;
}
.group .header {
    padding: 4px 8px;
    background-color: var(--background-color-hi);
    border-radius: 4px;
}
.panel {
    padding: 4px;
}
</style>
