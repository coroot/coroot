<template>
    <div>
        <div class="d-flex align-center mb-4">
            <div>
                <v-text-field
                    v-model="search"
                    label="search"
                    prepend-inner-icon="mdi-magnify"
                    dense
                    outlined
                    hide-details
                    clearable
                    class="search"
                />
            </div>
            <v-spacer />
            <v-menu offset-y max-width="500" :close-on-content-click="false">
                <template #activator="{ on }">
                    <v-icon v-on="on" color="primary" title="What are inspections?">mdi-help-circle-outline</v-icon>
                </template>
                <v-card class="pa-3">
                    <div class="body-2">
                        Inspections are automated checks that Coroot runs against your applications to detect potential issues.
                        Each inspection has a default threshold that triggers a warning when exceeded.
                        You can customize thresholds at the project level (applies to all applications) or override them for specific applications.
                    </div>
                </v-card>
            </v-menu>
        </div>

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="filteredChecks"
            sort-by="category"
            no-data-text="No inspections found"
            :headers="[
                { value: 'category', text: 'Category', sortable: true },
                { value: 'title', text: 'Inspection', sortable: true },
                { value: 'condition', text: 'Condition', sortable: false },
                { value: 'project_override', text: 'Project-level override', sortable: false },
                { value: 'application_overrides', text: 'Application-level overrides', sortable: false },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100] }"
        >

            <template #item.condition="{ item }">
                <span class="text-no-wrap">{{ formatCondition(item) }}</span>
            </template>

            <template #item.project_override="{ item }">
                <a @click="edit('::', item)">
                    <template v-if="item.project_threshold === null">
                        <v-icon small>mdi-file-replace-outline</v-icon>
                    </template>
                    <template v-else>
                        {{ format(item.project_threshold, item.unit, item.project_details) }}
                    </template>
                </a>
            </template>

            <template #item.application_overrides="{ item }">
                <div v-for="a in item.application_overrides" :key="a.id" class="text-no-wrap">
                    {{ $utils.appId(a.id).name }}:
                    <a @click="edit(a.id, item)">
                        {{ format(a.threshold, item.unit, a.details) }}
                    </a>
                </div>
                <a @click="openAppSelector(item)" title="Add application override">
                    <v-icon small>mdi-file-replace-outline</v-icon>
                </a>
            </template>
        </v-data-table>

        <CheckForm v-model="editing.active" :appId="editing.appId" :check="editing.check" />

        <v-dialog v-model="appSelector.active" max-width="500">
            <v-card class="pa-4">
                <div class="d-flex align-center mb-3">
                    <span class="text-subtitle-1 font-weight-medium">Select application</span>
                    <v-spacer />
                    <v-btn icon small @click="appSelector.active = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <v-autocomplete
                    v-model="appSelector.selectedApp"
                    :items="applications"
                    item-text="name"
                    item-value="id"
                    placeholder="Search application..."
                    dense
                    outlined
                    hide-details
                    autofocus
                    @change="onAppSelected"
                >
                    <template #item="{ item }">
                        <span>{{ item.name }}</span>
                        <span v-if="item.ns" class="caption grey--text ml-1">(ns: {{ item.ns }})</span>
                    </template>
                </v-autocomplete>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import CheckForm from './CheckForm.vue';

export default {
    components: { CheckForm },

    data() {
        return {
            checks: [],
            applications: [],
            search: this.$route.query.search || '',
            editing: {
                active: false,
            },
            appSelector: {
                active: false,
                check: null,
                selectedApp: null,
            },
        };
    },

    computed: {
        filteredChecks() {
            if (!this.search) {
                return this.checks;
            }
            const s = this.search.toLowerCase();
            return this.checks.filter((c) => {
                const category = (c.category || '').toLowerCase();
                const title = (c.title || '').toLowerCase();
                const condition = this.formatCondition(c).toLowerCase();
                return category.includes(s) || title.includes(s) || condition.includes(s);
            });
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        '$route.query.search'(newSearch) {
            this.search = newSearch || '';
        },
    },

    methods: {
        edit(appId, check) {
            this.editing = { active: true, appId, check };
        },
        openAppSelector(check) {
            this.appSelector = {
                active: true,
                check,
                selectedApp: null,
            };
        },
        onAppSelected(appId) {
            if (appId) {
                this.appSelector.active = false;
                this.edit(appId, this.appSelector.check);
            }
        },
        formatCondition(check) {
            return check.condition_format_template
                .replace('<bucket>', '500ms')
                .replace('<threshold>', this.format(check.global_threshold, check.unit));
        },
        format(threshold, unit, details) {
            if (threshold === null) {
                return '—';
            }
            let res = threshold;
            switch (unit) {
                case 'percent':
                    res = threshold + '%';
                    break;
                case 'second':
                    res = this.$format.duration(threshold * 1000, 'ms');
                    break;
            }
            if (details) {
                res += ' ' + details;
            }
            return res;
        },
        get() {
            this.$emit('loading', true);
            this.$api.getInspections((data, error) => {
                this.$emit('loading', false);
                if (error) {
                    this.$emit('error', error);
                    return;
                }
                this.checks = data.checks;
                const ctx = this.$api.context;
                const apps = (ctx.search && ctx.search.applications) || [];
                this.applications = apps.map((a) => {
                    const id = this.$utils.appId(a.id);
                    return {
                        id: a.id,
                        name: id.name,
                        ns: id.ns,
                    };
                }).sort((a, b) => a.name.localeCompare(b.name));
            });
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
.search:deep(input) {
    width: 200px !important;
}
</style>
