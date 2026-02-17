<template>
    <v-dialog v-model="dialog" max-width="600" scrollable>
        <v-card class="pa-4">
            <div class="d-flex align-center text-h6 mb-3">
                <div>Search for apps and nodes</div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <v-text-field v-model="search" dense outlined hide-details flat autofocus>
                <template #prepend-inner>
                    <v-icon color="grey">mdi-magnify</v-icon>
                </template>
            </v-text-field>

            <v-card-text class="px-0 mt-3" style="height: 500px">
                <v-alert v-if="error" color="error" text class="mt-3">
                    {{ error }}
                </v-alert>

                <div v-else-if="results.empty" class="text-center pa-3">No items found</div>

                <v-list v-else dense @click="dialog = false">
                    <template v-if="results.apps && results.apps.length">
                        <v-list-item class="px-0">
                            <v-list-item-icon class="mr-1"><v-icon small>mdi-apps</v-icon></v-list-item-icon>
                            <v-list-item-content>
                                <v-list-item-title>Applications</v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                        <v-list-item
                            v-for="a in results.apps"
                            :key="a.id"
                            :to="{ name: 'overview', params: { view: 'applications', id: a.id }, query: $utils.contextQuery() }"
                            class="pl-4"
                        >
                            <v-list-item-content>
                                <v-list-item-title>
                                    <Led :status="a.status" class="mr-1" />
                                    <span>{{ a.name }}</span>
                                    <span v-if="a.ns" class="caption"> (ns: {{ a.ns }})</span>
                                </v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                    </template>
                    <template v-if="results.nodes && results.nodes.length">
                        <v-list-item class="px-0">
                            <v-list-item-icon class="mr-1"><v-icon small>mdi-server</v-icon></v-list-item-icon>
                            <v-list-item-content>
                                <v-list-item-title>Nodes</v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                        <v-list-item
                            v-for="n in results.nodes"
                            :key="n.name"
                            :to="{ name: 'overview', params: { view: 'nodes', id: n.name }, query: $utils.contextQuery() }"
                            class="pl-4"
                        >
                            <v-list-item-content>
                                <v-list-item-title>
                                    <Led :status="n.status" class="mr-1" />
                                    <span>{{ n.name }}</span>
                                </v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                    </template>
                </v-list>
            </v-card-text>
        </v-card>
    </v-dialog>
</template>

<script>
import Led from '../components/Led.vue';

export default {
    props: {
        value: Boolean,
    },

    components: { Led },

    data() {
        return {
            context: this.$api.context,
            search: '',
            loading: false,
            error: '',
            dialog: this.value,
        };
    },

    watch: {
        dialog(v) {
            !v && this.$emit('input', false);
        },
    },

    computed: {
        results() {
            const items = this.context.search;
            if (!items) {
                return {};
            }
            const search = this.search.toLowerCase();
            const match = (s) => s.toLowerCase().includes(search);
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
};
</script>

<style scoped></style>
