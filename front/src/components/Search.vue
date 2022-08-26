<template>
    <v-menu :value="!!query" offset-y max-height="50vh">
        <template #activator="{ }">
            <v-text-field v-model="query" outlined hide-details dense flat placeholder="search for apps and nodes" @focus="open" @blur="close" @keydown.esc="close">
                <template #prepend-inner>
                    <v-icon color="grey">mdi-magnify</v-icon>
                </template>
            </v-text-field>
        </template>
        <v-list dark color="#080d1b">
            <template v-if="error">
                <v-list-item>
                    <v-list-item-title class="ml-3">
                        {{error}}
                    </v-list-item-title>
                </v-list-item>
            </template>
            <template v-if="results.empty">
                <v-list-item>
                    <v-list-item-title class="ml-3">
                        no entries found
                    </v-list-item-title>
                </v-list-item>
            </template>
            <template v-if="results.apps && results.apps.length">
                <v-list-item>
                    <v-list-item-icon class="mr-1"><v-icon small>mdi-apps</v-icon></v-list-item-icon>
                    <v-list-item-content>
                        <v-list-item-title>Applications</v-list-item-title>
                    </v-list-item-content>
                </v-list-item>
                <v-list-item v-for="a in results.apps" :to="{name: 'application', params: {id: a.id}, query: $route.query}">
                    <v-list-item-title class="ml-3">
                        <Led :status="a.status" />
                        {{$api.appId(a.id).name}}
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
                <v-list-item v-for="n in results.nodes" :to="{name: 'node', params: {name: n.name}, query: $route.query}">
                    <v-list-item-title class="ml-3">
                        <Led :status="n.status" />
                        {{n.name}}
                    </v-list-item-title>
                </v-list-item>
            </template>
        </v-list>
    </v-menu>
</template>

<script>
import Led from "@/components/Led";

export default {
    components: {Led},

    data() {
        return {
            apps: null,
            nodes: null,
            query: '',
            loading: false,
            error: '',
        };
    },

    computed: {
        results() {
            if (!this.query) {
                return {};
            }
            const q = this.query.toLowerCase();
            const match = (s) => s.toLowerCase().includes(q);
            const apps = (this.apps || []).filter((a) => match(a.id));
            const nodes = (this.nodes || []).filter((n) => match(n.name));
            const empty = !apps.length && !nodes.length;
            return {apps, nodes, empty};
        },

    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.search((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.apps = data.applications;
                this.nodes = data.nodes;
            });
        },
        open() {
            this.get();
        },
        close() {
            setTimeout(() => {
                this.query = '';
            }, 200)
        },
    },
}
</script>

<style scoped>

</style>
