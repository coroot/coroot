<template>
    <div>
        <v-simple-table>
            <thead>
                <tr>
                    <th>Category</th>
                    <th>Patterns</th>
                    <th>Notify of deployments</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="c in categories">
                    <td class="text-no-wrap">
                        <div class="text-no-wrap">{{ c.name }}</div>
                    </td>
                    <td style="line-height: 2em">
                        <div v-if="c.default" class="grey--text">
                            The default category containing applications that don't fit into other categories
                        </div>
                        <template v-else v-for="p in (c.builtin_patterns + ' ' + c.custom_patterns).split(' ').filter((p) => !!p)">
                            <span class="pattern">{{ p }}</span>
                            &nbsp;
                        </template>
                    </td>
                    <td>
                        {{ c.notify_of_deployments ? 'on' : 'off' }}
                    </td>
                    <td>
                        <div class="d-flex">
                            <v-btn icon small @click="openForm(c)"><v-icon small>mdi-pencil</v-icon></v-btn>
                            <v-btn v-if="!c.builtin" icon small @click="openForm(c, true)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                        </div>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>

        <v-btn color="primary" class="mt-2" @click="openForm()" small>Add a category</v-btn>

        <v-dialog v-model="form.active" max-width="800">
            <v-card class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    <div v-if="form.new">Add a new application category</div>
                    <div v-else-if="form.del">Delete the "{{ form.name }}" application category</div>
                    <div v-else>Edit the "{{ form.name }}" application category</div>
                    <v-spacer />
                    <v-btn icon @click="form.active = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>

                <v-form v-model="form.valid" ref="form">
                    <div class="subtitle-1">Name</div>
                    <v-text-field v-model="form.name" outlined dense :disabled="form.builtin || form.del" :rules="[$validators.isSlug]" />

                    <template v-if="!form.default">
                        <template v-if="form.builtin">
                            <div class="subtitle-1">Built-in patterns</div>
                            <v-textarea v-model="form.builtin_patterns" outlined dense rows="1" auto-grow disabled />
                        </template>

                        <div class="subtitle-1">Custom patterns</div>
                        <div class="caption">
                            space-delimited list of
                            <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                            in the <var>&lt;namespace&gt;/&lt;application_name&gt;</var> format, e.g.: <var>staging/* test-*/*</var>
                        </div>
                        <v-textarea v-model="form.custom_patterns" outlined dense rows="1" auto-grow :disabled="form.del" hide-details />
                    </template>

                    <v-checkbox
                        v-model="form.notify_of_deployments"
                        :disabled="form.del"
                        label="Get notified of deployments"
                        class="my-2"
                        hide-details
                    />
                    <div v-if="form.notify_of_deployments" class="my-2">
                        <ul v-if="integrations && Object.keys(integrations).length">
                            <li v-for="(details, type) in integrations">
                                <span>{{ type }}</span>
                                <span v-if="details" class="grey--text"> ({{ details }})</span>
                            </li>
                        </ul>
                        <div v-else class="grey--text">No notification integrations configured.</div>
                        <v-btn
                            color="primary"
                            small
                            :to="{ name: 'project_settings', params: { tab: 'notifications' } }"
                            @click="form.active = false"
                            class="mt-1"
                        >
                            Configure integrations
                        </v-btn>
                    </div>

                    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                        {{ error }}
                    </v-alert>
                    <v-alert v-if="message" color="green" outlined text>
                        {{ message }}
                    </v-alert>
                    <div class="d-flex align-center">
                        <v-spacer />
                        <v-btn v-if="form.del" color="error" :loading="saving" @click="save">Delete</v-btn>
                        <v-btn v-else color="primary" :disabled="!form.valid" :loading="saving" @click="save">Save</v-btn>
                    </div>
                </v-form>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
export default {
    props: {
        projectId: String,
    },

    data() {
        return {
            categories: [],
            integrations: [],
            loading: false,
            error: '',
            message: '',
            form: {
                builtin: false,
                default: false,
                active: false,
                new: false,
                del: false,
                oldName: '',
                name: '',
                builtin_patterns: '',
                custom_patterns: '',
                notify_of_deployments: false,
                valid: true,
            },
            saving: false,
        };
    },

    mounted() {
        this.get();
    },

    watch: {
        projectId() {
            this.get();
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getApplicationCategories((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.categories = data.categories;
                this.integrations = data.integrations;
                const category = this.categories ? this.categories.find((c) => c.name === this.$route.query.category) || {} : {};
                const p = this.$route.query.app_pattern;
                if (!category.name && !p) {
                    return;
                }
                if (p) {
                    if (!category.custom_patterns) {
                        category.custom_patterns = p;
                    } else {
                        category.custom_patterns += ' ' + p;
                    }
                }
                this.openForm(category);
                this.$router.replace({ query: { ...this.$route.query, category: undefined, app_pattern: undefined }, hash: this.$route.hash });
            });
        },
        openForm(category, del) {
            this.error = '';
            this.form.builtin = category && category.builtin;
            this.form.default = category && category.default;
            this.form.active = true;
            this.form.new = !category || !category.name;
            this.form.del = del;
            this.form.oldName = category ? category.name : '';
            this.form.name = category ? category.name : '';
            this.form.builtin_patterns = category ? category.builtin_patterns : '';
            this.form.custom_patterns = category ? category.custom_patterns : '';
            this.form.notify_of_deployments = category && category.notify_of_deployments;
            this.$refs.form && this.$refs.form.resetValidation();
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            const patterns = this.form.del ? '' : this.form.custom_patterns;
            const form = {
                name: this.form.oldName,
                new_name: this.form.name,
                custom_patterns: patterns,
                notify_of_deployments: this.form.notify_of_deployments,
            };
            this.$api.saveApplicationCategory(form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                    this.form.active = false;
                }, 1000);
                this.get();
            });
        },
    },
};
</script>

<style scoped>
.pattern {
    border: 1px solid #bdbdbd;
    border-radius: 4px;
    padding: 2px 4px;
    white-space: nowrap;
}
</style>
