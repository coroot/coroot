<template>
    <div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-2">
            {{ error }}
        </v-alert>
        <v-alert v-if="disabled" color="info" outlined text>
            Coroot Community Edition includes three predefined roles: Admin, Editor, and Viewer.
            <br />
            For more granular Role-Based Access Control (RBAC), upgrade to Coroot Enterprise (from $1 per CPU core/month).
            <a href="https://coroot.com/account" target="_blank" class="font-weight-bold">Start</a> your free trial today.
        </v-alert>
        <v-simple-table v-if="!error" dense class="table mt-5">
            <thead>
                <tr>
                    <th>Action</th>
                    <th v-for="r in roles">
                        <div class="d-flex">
                            <div>
                                <span>{{ r.name }}</span>
                                <span v-if="disabled && r.custom">*</span>
                            </div>
                            <div class="d-flex align-center">
                                <v-btn v-if="r.custom" @click="edit(r)" x-small icon><v-icon x-small>mdi-pencil</v-icon></v-btn>
                            </div>
                        </div>
                    </th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="a in actions">
                    <td>{{ a.name }}</td>
                    <td v-for="r in a.roles">
                        <v-icon v-if="!r.objects" small color="red">mdi-close-thick</v-icon>
                        <v-icon v-else-if="!r.objects.length" small color="green">mdi-check-bold</v-icon>
                        <v-tooltip v-else bottom>
                            <template #activator="{ on }">
                                <v-icon v-on="on" small color="green">mdi-list-status</v-icon>
                            </template>
                            <v-card class="pa-2">
                                <div v-for="o in r.objects">{{ o }}</div>
                            </v-card>
                        </v-tooltip>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>
        <v-btn v-if="!error" color="primary" @click="add()" small :disabled="disabled" class="mt-3">Add role</v-btn>
        <div v-if="disabled" class="mt-2 grey--text">* - examples of fine-grained custom roles</div>

        <v-dialog v-model="form.active" max-width="800">
            <v-card class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    {{ form.title }}
                    <v-btn v-if="form.action === 'edit'" :disabled="disabled" @click="form.action = 'delete'" icon small>
                        <v-icon small>mdi-trash-can-outline</v-icon>
                    </v-btn>
                    <v-spacer />
                    <v-btn icon @click="form.active = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <v-form v-model="form.valid" :disabled="disabled" ref="form" class="form">
                    <div class="font-weight-medium">Name</div>
                    <v-text-field v-model="form.name" outlined dense :rules="[$validators.notEmpty]" />
                    <div class="font-weight-medium">Permission policies</div>
                    <v-simple-table dense class="mb-4 mt-2">
                        <thead>
                            <tr>
                                <th style="width: 40%">Scope</th>
                                <th style="width: 15%">Action</th>
                                <th>Object</th>
                                <th style="width: 5%"></th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="(p, i) in form.permissions">
                                <td>
                                    <v-select
                                        v-model="p.scope"
                                        :items="scopes.map((s) => s.name)"
                                        outlined
                                        dense
                                        hide-details
                                        :menu-props="{ offsetY: true }"
                                        :rules="[$validators.notEmpty]"
                                    />
                                </td>
                                <td>
                                    <v-select
                                        v-model="p.action"
                                        :items="(scopes.find((s) => s.name === p.scope) || {}).actions"
                                        outlined
                                        dense
                                        hide-details
                                        :menu-props="{ offsetY: true }"
                                        :rules="[$validators.notEmpty]"
                                    />
                                </td>
                                <td>
                                    <v-text-field v-model="p.object" outlined dense hide-details />
                                </td>
                                <td>
                                    <v-btn small icon :disabled="disabled" @click="form.permissions.splice(i, 1)">
                                        <v-icon small>mdi-trash-can-outline</v-icon>
                                    </v-btn>
                                </td>
                            </tr>
                        </tbody>
                        <tfoot>
                            <v-btn
                                color="primary"
                                small
                                class="ml-1 mt-2"
                                :disabled="disabled"
                                @click="form.permissions.push({ scope: '', action: '', object: '' })"
                            >
                                Add policy
                            </v-btn>
                        </tfoot>
                    </v-simple-table>
                    <div v-if="disabled" class="mb-2 caption grey--text">
                        This form is disabled because adjusting role permissions is not supported in the Coroot Community Edition.
                    </div>
                    <v-alert v-if="form.error" color="red" icon="mdi-alert-octagon-outline" outlined text>{{ form.error }}</v-alert>
                    <v-alert v-if="form.message" color="green" outlined text>{{ form.message }}</v-alert>
                    <div class="d-flex align-center">
                        <v-spacer />
                        <template v-if="form.action === 'delete'">
                            <div>Are you sure you want to delete the role?</div>
                            <v-btn color="error" :disabled="disabled" @click="post" :loading="form.loading" small class="ml-2">Delete</v-btn>
                            <v-btn color="info" @click="form.action = 'edit'" small class="ml-2">Cancel</v-btn>
                        </template>
                        <template v-else>
                            <v-btn color="primary" :disabled="disabled || !form.valid" @click="post" :loading="form.loading">Save</v-btn>
                        </template>
                    </div>
                </v-form>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
export default {
    data() {
        return {
            loading: false,
            error: '',
            disabled: this.$coroot.edition !== 'Enterprise',
            roles: [],
            actions: [],
            scopes: [],
            form: {
                active: false,
                valid: true,
                loading: false,
                error: '',
                message: '',
                title: '',
                action: '',
                id: '',
                name: '',
                permissions: [],
            },
        };
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.roles(null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.roles = data.roles || [];
                this.actions = data.actions || [];
                this.scopes = data.scopes || [];
            });
        },
        post() {
            const { action, id, name, permissions } = this.form;
            this.form.loading = true;
            const form = { action, id, name, permissions };
            this.$api.roles(form, (data, error) => {
                this.form.loading = false;
                if (error) {
                    this.form.error = error;
                    return;
                }
                this.$events.emit('roles');
                this.form.active = false;
                this.get();
            });
        },
        add() {
            this.form.message = '';
            this.form.error = '';
            this.form.active = true;
            this.form.title = 'Add role';
            this.form.action = 'add';
            this.form.name = '';
            this.form.permissions = [{ scope: '', action: '', object: '' }];
            this.$refs.form && this.$refs.form.resetValidation();
        },
        edit(role) {
            this.form.message = '';
            this.form.error = '';
            this.form.active = true;
            this.form.title = 'Edit role';
            this.form.action = 'edit';
            this.form.id = role.name;
            this.form.name = role.name;
            this.form.permissions = (role.permissions || []).map(({ scope, action, object }) => {
                return { scope, action, object: object ? JSON.stringify(object) : '*' };
            });
            this.$refs.form && this.$refs.form.resetValidation();
        },
    },
};
</script>

<style scoped>
.table:deep(th),
.table:deep(td) {
    padding: 0 8px !important;
}

.form:deep(table) {
    table-layout: fixed;
    min-width: 600px;
}
.form:deep(tr:hover) {
    background-color: unset !important;
}
.form:deep(th) {
    height: unset !important;
    padding: 0 8px !important;
    border-bottom: none !important;
}
.form:deep(td) {
    padding: 4px !important;
    border-bottom: none !important;
}
.form:deep(.v-input__slot) {
    min-height: initial !important;
    height: 32px !important;
    padding: 0 8px !important;
}
.form:deep(.v-input__append-inner) {
    margin-top: 4px !important;
}
*:deep(.v-list-item) {
    min-height: 32px !important;
    padding: 0 8px !important;
}
</style>
