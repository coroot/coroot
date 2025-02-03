<template>
    <div class="d-flex flex-column flex-sm-row flex-wrap flex-md-nowrap" style="gap: 12px">
        <div>
            <v-text-field
                v-model="searchString"
                label="search"
                clearable
                dense
                hide-details
                prepend-inner-icon="mdi-magnify"
                outlined
                class="search"
            />
        </div>

        <div>
            <v-autocomplete
                :items="namespaces"
                v-model="selectedNamespaces"
                label="namespaces"
                :disabled="namespacesDisabled"
                color="primary"
                multiple
                outlined
                dense
                chips
                small-chips
                deletable-chips
                hide-details
                class="namespaces"
                :class="{ empty: !selectedNamespaces.length }"
            >
                <template #selection="{ item }">
                    <v-chip
                        small
                        label
                        :disabled="namespacesDisabled"
                        close
                        close-icon="mdi-close"
                        @click:close="removeNamespace(item.value)"
                        color="primary"
                        class="namespace"
                    >
                        <span :title="item.text">{{ item.name }}</span>
                    </v-chip>
                </template>
            </v-autocomplete>
        </div>

        <div class="d-none d-sm-block flex-grow-1" />

        <div class="d-flex flex-wrap align-center categories">
            <v-checkbox
                v-for="c in categories"
                :key="c"
                :value="c"
                v-model="selectedCategories"
                :label="c"
                :disabled="categoriesDisabled"
                class="category"
                color="primary"
                hide-details
            />

            <v-tooltip bottom>
                <template #activator="{ on }">
                    <v-btn :to="{ name: 'project_settings', params: { tab: 'applications' } }" v-on="on" icon x-small>
                        <v-icon>mdi-plus</v-icon>
                    </v-btn>
                </template>
                <v-card class="px-2">configure categories</v-card>
            </v-tooltip>
        </div>
    </div>
</template>

<script>
const storageKey = 'application-filter';

function autoSelectNamespace(namespaces, maxApps) {
    let ns = namespaces.find((ns) => ns.value === 'default');
    if (ns && ns.apps <= maxApps) {
        return ns.value;
    }
    let name = '';
    let apps = 0;
    namespaces.forEach((ns) => {
        if (ns.value && ns.apps > apps && ns.apps <= maxApps) {
            name = ns.value;
            apps = ns.apps;
        }
    });
    if (name) {
        return name;
    }
    return namespaces[0].value;
}

export default {
    props: {
        applications: Array,
        autoSelectNamespaceThreshold: Number,
    },

    data() {
        const saved = this.load();
        return {
            selectedCategories: saved.categories,
            selectedNamespaces: saved.namespaces,
            searchString: '',
            autoSelectNamespace: !!this.autoSelectNamespaceThreshold && !saved.namespaces.length,
        };
    },

    computed: {
        search() {
            return (this.searchString || '').trim();
        },
        categories() {
            const set = new Set(this.selectedCategories);
            (this.applications || []).forEach((a) => {
                set.add(a.category);
            });
            const categories = Array.from(set);
            categories.sort((a, b) => a.localeCompare(b));
            return categories;
        },
        categoriesDisabled() {
            return !!this.search || !!this.selectedNamespaces.length;
        },
        namespaces() {
            const map = {};
            this.selectedNamespaces.forEach((ns) => {
                map[ns] = 0;
            });
            (this.applications || []).forEach((a) => {
                const id = this.$utils.appId(a.id);
                if (!map[id.ns]) {
                    map[id.ns] = 0;
                }
                map[id.ns]++;
            });
            const namespaces = Object.keys(map).map((ns) => {
                const name = ns || '~empty';
                const apps = map[ns];
                return { value: ns, name, apps, text: `${name} (${apps})` };
            });
            namespaces.sort((a, b) => a.value.localeCompare(b.value));
            return namespaces;
        },
        namespacesDisabled() {
            return !!this.search;
        },
        filter() {
            const selectedCategories = new Set(this.selectedCategories);
            const selectedNamespaces = new Set(this.selectedNamespaces);
            const search = this.search;
            const applications = (this.applications || []).filter((a) => {
                if (search) {
                    return a.id.includes(search) || (a.type && a.type.name.includes(search));
                }
                if (selectedNamespaces.size) {
                    return selectedNamespaces.has(this.$utils.appId(a.id).ns);
                }
                return selectedCategories.has(a.category);
            });
            return new Set(applications.map((a) => a.id));
        },
    },

    watch: {
        filter: {
            handler() {
                if (!this.selectedCategories.length && this.categories.length) {
                    this.selectedCategories.push(this.categories[0]);
                    this.save();
                    return;
                }
                if (
                    this.autoSelectNamespace &&
                    this.filter.size > this.autoSelectNamespaceThreshold &&
                    !this.selectedNamespaces.length &&
                    this.namespaces
                ) {
                    this.selectedNamespaces.push(autoSelectNamespace(this.namespaces, this.autoSelectNamespaceThreshold));
                    this.autoSelectNamespace = false;
                    this.save();
                    return;
                }
                this.$emit('filter', this.filter);
            },
            immediate: true,
        },
        selectedCategories() {
            this.save();
        },
        selectedNamespaces() {
            this.save();
        },
    },

    methods: {
        removeNamespace(ns) {
            const i = this.selectedNamespaces.indexOf(ns);
            if (i >= 0) {
                this.selectedNamespaces.splice(i, 1);
            }
        },
        load() {
            const projectId = this.$route.params.projectId;
            let saved = this.$storage.local(storageKey) || {};
            saved = saved[projectId] || {};
            saved.categories = saved.categories || [];
            saved.namespaces = saved.namespaces || [];
            return saved;
        },
        save() {
            const saved = this.$storage.local(storageKey) || {};
            const projectId = this.$route.params.projectId;
            if (!saved[projectId]) {
                saved[projectId] = {};
            }
            saved[projectId].categories = this.selectedCategories;
            saved[projectId].namespaces = this.selectedNamespaces;
            this.$storage.local(storageKey, saved);
        },
    },
};
</script>

<style scoped>
.categories {
}
.category {
    white-space: nowrap;
    margin: 0 12px 0 0;
    padding: 0;
}
.category:deep(.v-input--selection-controls__input) {
    margin-right: 2px !important;
}
*:deep(.v-list-item) {
    font-size: 14px !important;
    padding: 0 8px !important;
}
*:deep(.v-list-item__action) {
    margin: 4px !important;
}
.namespaces:deep(input) {
    width: 0 !important;
}
.namespaces.empty:deep(input) {
    width: 100px !important;
}
.namespace {
    margin: 4px 4px 0 0 !important;
    padding: 0 8px !important;
}
.namespace span {
    max-width: 20ch;
    overflow: hidden;
    text-overflow: ellipsis;
}
.namespace:deep(.v-icon) {
    font-size: 16px !important;
}
.search:deep(input) {
    width: 100px !important;
}
</style>
