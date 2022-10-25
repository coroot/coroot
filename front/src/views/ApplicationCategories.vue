<template>
<div>
    <v-simple-table>
        <thead>
        <tr>
            <th>Category</th>
            <th>Patterns</th>
            <th>Actions</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="c in categories">
            <td class="text-no-wrap">{{ c.name }}</td>
            <td style="line-height: 2em">
                <template v-for="p in (c.builtin_patterns + ' ' +  c.custom_patterns).split(' ').filter(p => !!p)">
                    <span class="pattern">{{ p }}</span>&nbsp;
                </template>
            </td>
            <td>
                <div class="d-flex">
                    <v-btn icon small @click="openForm(c)"><v-icon small>mdi-pencil</v-icon></v-btn>
                    <v-btn v-if="!c.builtin_patterns" icon small @click="openForm(c, true)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                </div>
            </td>
        </tr>
        </tbody>
    </v-simple-table>

    <v-btn color="primary" class="mt-2" @click="openForm()">Add a category</v-btn>

    <v-dialog v-model="edit.active" max-width="800">
        <v-card class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-4">
                <div v-if="edit.new">
                    Add a new application category
                </div>
                <div v-else-if="edit.del">
                    Delete the "{{edit.name}}" application category
                </div>
                <div v-else>
                    Edit the "{{edit.name}}" application category
                </div>
                <v-spacer />
                <v-btn icon @click="edit.active = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <v-form v-model="edit.valid" ref="form">
                <div class="subtitle-1">Category name</div>
                <v-text-field v-model="edit.name" outlined dense :disabled="!!edit.builtin_patterns || edit.del" :rules="[$validators.isSlug]" />

                <template v-if="edit.builtin_patterns">
                    <div class="subtitle-1">Built-in patterns</div>
                    <v-textarea v-model="edit.builtin_patterns" outlined dense rows="1" auto-grow disabled />
                </template>

                <div class="subtitle-1">Custom patterns</div>
                <div class="caption">
                    space-delimited list of
                    <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                    in the <var>&lt;namespace&gt;/&lt;application_name&gt;</var> format
                    , e.g.: <var>staging/* test-*/*</var>
                </div>
                <v-textarea v-model="edit.custom_patterns" outlined dense rows="1" auto-grow :disabled="edit.del"/>
                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                    {{error}}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text>
                    {{message}}
                </v-alert>
                <div class="d-flex align-center">
                    <v-spacer />
                    <v-btn v-if="edit.del" color="error" :loading="saving" @click="save">Delete</v-btn>
                    <v-btn v-else color="primary" :disabled="!edit.valid" :loading="saving" @click="save">Save</v-btn>
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
            loading: false,
            error: '',
            message: '',
            edit: {
                active: false,
                new: false,
                del: false,
                oldName: '',
                name: '',
                builtin_patterns: '',
                custom_patterns: '',
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
        }
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
            });
        },
        openForm(category, del) {
            this.error = '';
            this.edit.active = true;
            this.edit.new = !category;
            this.edit.del = del;
            this.edit.oldName = category ? category.name : ''
            this.edit.name = category ? category.name : '';
            this.edit.builtin_patterns = category ? category.builtin_patterns  : '';
            this.edit.custom_patterns = category ? category.custom_patterns : '';
            this.$refs.form && this.$refs.form.resetValidation();
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            const patterns = this.edit.del ? '' : this.edit.custom_patterns;
            const form = {name: this.edit.oldName, new_name: this.edit.name, custom_patterns: patterns};
            this.$api.saveApplicationCategory(form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                    this.edit.active = false;
                }, 1000);
                this.get();
            });
        },
    },
}
</script>

<style scoped>
.pattern {
    border: 1px solid #BDBDBD;
    border-radius: 4px;
    padding: 2px 4px;
    white-space: nowrap;
}
</style>