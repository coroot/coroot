<template>
    <div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-simple-table v-if="!error" dense>
            <thead>
                <tr>
                    <th>Email (Login)</th>
                    <th>Name</th>
                    <th>Role</th>
                    <th></th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="u in users">
                    <td>{{ u.email }}</td>
                    <td>{{ u.name }}</td>
                    <td>{{ u.role }}</td>
                    <td>
                        <template v-if="!u.readonly">
                            <v-btn small icon @click="update(u)"><v-icon small>mdi-pencil</v-icon></v-btn>
                            <v-btn small icon @click="del(u)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                        </template>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>
        <v-btn v-if="!error" color="primary" small class="mt-3" @click="create(false)">Add user</v-btn>

        <v-dialog v-model="form.active" max-width="600">
            <v-card class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    {{ form.title }}
                    <v-spacer />
                    <v-btn icon @click="form.active = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <v-form v-model="form.valid" ref="form">
                    <div class="font-weight-medium">Email (Login)</div>
                    <v-text-field
                        outlined
                        dense
                        type="email"
                        v-model="form.email"
                        name="email"
                        :disabled="form.readonly"
                        :rules="[$validators.isEmail]"
                    />

                    <div class="font-weight-medium">Name</div>
                    <v-text-field outlined dense v-model="form.name" name="name" :disabled="form.readonly" :rules="[$validators.notEmpty]" />

                    <div class="font-weight-medium">Role</div>
                    <v-select
                        v-model="form.role"
                        :items="roles"
                        name="role"
                        :disabled="form.readonly"
                        outlined
                        dense
                        :menu-props="{ offsetY: true }"
                        :rules="[$validators.notEmpty]"
                    />

                    <div class="font-weight-medium">Password</div>
                    <v-text-field
                        outlined
                        dense
                        type="password"
                        v-model="form.password"
                        name="password"
                        :disabled="form.readonly"
                        :rules="form.action === 'create' ? [$validators.notEmpty] : []"
                    />

                    <v-alert v-if="form.error" color="red" icon="mdi-alert-octagon-outline" outlined text>{{ form.error }}</v-alert>
                    <v-alert v-if="form.message" color="green" outlined text>{{ form.message }}</v-alert>
                    <div class="d-flex align-center">
                        <v-spacer />
                        <v-btn :color="form.button.color" :disabled="!form.valid" :loading="form.loading" @click="post">{{ form.button.text }}</v-btn>
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
            users: [],
            roles: [],
            loading: false,
            error: '',
            message: '',
            form: {
                active: false,
                loading: false,
                valid: true,
                readonly: false,

                error: '',
                message: '',

                title: '',
                button: { text: '', color: 'primary' },

                action: '',
                id: 0,
                name: '',
                email: '',
                role: '',
                password: '',
            },
        };
    },

    mounted() {
        this.$events.watch(this, this.get, 'roles');
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.users(null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.users = data.users || [];
                this.roles = data.roles || [];
            });
        },
        post() {
            this.form.loading = true;
            const { id, action, name, email, role, password } = this.form;
            this.$api.users({ id, action, name, email, role, password }, (data, error) => {
                this.form.loading = false;
                if (error) {
                    this.form.error = error;
                    return;
                }
                this.form.active = false;
                this.get();
            });
        },
        create() {
            this.form.message = '';
            this.form.error = '';
            this.form.active = true;
            this.form.readonly = false;
            this.form.title = 'New user';
            this.form.button.text = 'Create';
            this.form.button.color = 'primary';
            this.form.action = 'create';
            this.form.name = '';
            this.form.email = '';
            this.form.role = '';
            this.form.password = '';
            this.$refs.form && this.$refs.form.resetValidation();
        },
        update(user) {
            this.form.message = '';
            this.form.error = '';
            this.form.active = true;
            this.form.readonly = false;
            this.form.title = 'Edit user';
            this.form.button.text = 'Save';
            this.form.button.color = 'primary';
            this.form.action = 'update';
            this.form.id = user.id;
            this.form.name = user.name;
            this.form.email = user.email;
            this.form.role = user.role;
            this.form.password = '';
            this.$refs.form && this.$refs.form.resetValidation();
        },
        del(user) {
            this.form.message = '';
            this.form.error = '';
            this.form.active = true;
            this.form.readonly = true;
            this.form.title = 'Delete user';
            this.form.button.text = 'Delete';
            this.form.button.color = 'error';
            this.form.action = 'delete';
            this.form.id = user.id;
            this.form.name = user.name;
            this.form.email = user.email;
            this.form.role = user.role;
            this.form.password = '';
            this.$refs.form && this.$refs.form.resetValidation();
        },
    },
};
</script>

<style scoped></style>
