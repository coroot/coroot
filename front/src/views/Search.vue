<template>
    <v-menu :value="!!query" offset-y max-height="50vh" attach=".v-app-bar">
        <template #activator="{}">
            <v-text-field v-model="query" dense outlined hide-details flat placeholder="search for apps and nodes" @blur="close" @keydown.esc="close">
                <template #prepend-inner>
                    <v-icon color="grey">mdi-magnify</v-icon>
                </template>
            </v-text-field>
        </template>
        <v-list dark>
            <template v-if="error">
                <v-list-item>
                    <v-list-item-title class="ml-3">
                        {{ error }}
                    </v-list-item-title>
                </v-list-item>
            </template>
            <template v-if="results.empty">
                <v-list-item>
                    <v-list-item-title class="ml-3"> no entries found </v-list-item-title>
                </v-list-item>
            </template>
            <template v-if="results.apps && results.apps.length">
                <v-list-item>
                    <v-list-item-icon class="mr-1"><v-icon small>mdi-apps</v-icon></v-list-item-icon>
                    <v-list-item-content>
                        <v-list-item-title>Applications</v-list-item-title>
                    </v-list-item-content>
                </v-list-item>
                <v-list-item
                    v-for="a in results.apps"
                    :key="a.id"
                    :to="{ name: 'overview', params: { view: 'applications', id: a.id }, query: $utils.contextQuery() }"
                >
                    <v-list-item-title class="ml-3">
                        <Led :status="a.status" />
                        <span>{{ a.name }}</span>
                        <span v-if="a.ns" class="caption"> (ns: {{ a.ns }})</span>
                    </v-list-item-title>
                </v-list-item>
            </template>
            <template v-if="results.nodes && results.nodes.length">
                <v-list-item>
                    <v-list-item-icon class="mr-1"><v-icon small>mdi-server</v-icon></v-list-item-icon>
                    <v-list-item-content>
                        <v-list-item-title>Nodes</v-list-item-title>
                    </v-list-item-content>
                </v-list-item>
                <v-list-item
                    v-for="n in results.nodes"
                    :key="n.name"
                    :to="{ name: 'overview', params: { view: 'nodes', id: n.name }, query: $utils.contextQuery() }"
                >
                    <v-list-item-title class="ml-3">
                        <Led :status="n.status" />
                        <span>{{ n.name }}</span>
                    </v-list-item-title>
                </v-list-item>
            </template>
        </v-list>
    </v-menu>
</template>

<script>
import Led from '../components/Led.vue';

export default {
    components: { Led },

    data() {
        return {
            context: this.$api.context,
            query: '',
            loading: false,
            error: '',
        };
    },

    computed: {
        results() {
            const items = this.context.search;
            if (!this.query || !items) {
                return {};
            }
            const q = this.query.toLowerCase();
            const match = (s) => s.toLowerCase().includes(q);
            const apps = (items.applications || [])
                .filter((a) => match(a.id))
                .map((a) => {
                    const id = this.$utils.appId(a.id);
                    return {
                        id: a.id,
                        name: id.name,
                        ns: id.ns,
                        status: a.status,
                    };
                });
            apps.sort((a1, a2) => a1.name.localeCompare(a2.name));
            const nodes = (items.nodes || []).filter((n) => match(n.name));
            nodes.sort((n1, n2) => n1.name.localeCompare(n2.name));
            const empty = !apps.length && !nodes.length;
            return { apps, nodes, empty };
        },
    },

    methods: {
        close() {
            setTimeout(() => {
                this.query = '';
            }, 200);
        },
    },
};
</script>

<style scoped></style>
