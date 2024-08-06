<template>
    <div>
        <v-alert color="info" outlined text>
            Coroot Community Edition includes three predefined roles: Admin, Editor, and Viewer.
            <br />
            For more granular Role-Based Access Control (RBAC), upgrade to Coroot Enterprise.
            <a href="https://coroot.com/contact/" target="_blank" class="font-weight-bold">Contact us</a> for a free trial.
        </v-alert>
        <v-simple-table dense class="mt-5">
            <thead>
                <tr>
                    <th>Action</th>
                    <th v-for="r in roles" class="text-no-wrap">
                        <span :class="{ 'grey--text': r.custom }">{{ r.name }}</span>
                        <span v-if="r.custom">*</span>
                        <v-btn v-if="r.editable" @click="edit(r)" small icon>
                            <v-icon x-small>mdi-pencil</v-icon>
                        </v-btn>
                    </th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="a in matrix">
                    <td>{{ a.name }}</td>
                    <td v-for="r in a.roles">
                        <v-icon v-if="r === '*'" small color="green">mdi-check-bold</v-icon>
                        <v-icon v-else-if="r === ''" small color="red">mdi-close-thick</v-icon>
                        <v-tooltip v-else bottom>
                            <template #activator="{ on }">
                                <v-icon v-on="on" small color="green">mdi-list-status</v-icon>
                            </template>
                            <v-card class="pa-2">{{ r }}</v-card>
                        </v-tooltip>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>
        <v-btn color="primary" disabled class="mt-3">Add role</v-btn>
        <div class="mt-2 grey--text">* - examples of fine-grained custom roles</div>

        <v-dialog v-model="form.active" max-width="800">
            <v-card class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    Edit role '{{ form.name }}'
                    <v-spacer />
                    <v-btn icon @click="form.active = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <v-form ref="form" class="form">
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
                            <tr v-for="p in form.permissions">
                                <td>
                                    <v-select
                                        v-model="p.scope"
                                        :items="scopes"
                                        outlined
                                        dense
                                        hide-details
                                        :menu-props="{ offsetY: true }"
                                        disabled
                                    />
                                </td>
                                <td>
                                    <v-select
                                        v-model="p.action"
                                        :items="actions"
                                        outlined
                                        dense
                                        hide-details
                                        :menu-props="{ offsetY: true }"
                                        disabled
                                    />
                                </td>
                                <td>
                                    <v-text-field v-model="p.object" outlined dense hide-details disabled />
                                </td>
                                <td>
                                    <v-btn small icon disabled>
                                        <v-icon small>mdi-trash-can-outline</v-icon>
                                    </v-btn>
                                </td>
                            </tr>
                        </tbody>
                        <tfoot>
                            <v-btn color="primary" small class="ml-1 mt-1" disabled>Add policy</v-btn>
                        </tfoot>
                    </v-simple-table>
                    <div class="mb-2 caption grey--text">
                        This form is disabled because adjusting role permissions is not supported in the Coroot Community Edition.
                    </div>
                    <div class="d-flex align-center">
                        <v-spacer />
                        <v-btn color="primary" disabled>Save</v-btn>
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
            form: {
                active: false,
                name: '',
                permissions: [],
            },
        };
    },
    computed: {
        actions() {
            return ['*', 'view', 'edit'];
        },
        scopes() {
            return [
                '*',
                'users',
                'project.*',
                'project.settings',
                'project.integrations',
                'project.application_categories',
                'project.custom_applications',
                'project.inspections',
                'project.instrumentations',
                'project.traces',
                'project.costs',
                'project.application',
                'project.node',
            ];
        },
        roles() {
            return [
                { name: 'Admin', editable: false, permissions: [{ scope: '*', action: '*', object: '*' }] },
                {
                    name: 'Editor',
                    editable: true,
                    permissions: [
                        { scope: '*', action: 'view', object: '*' },
                        { scope: 'project.application_categories', action: 'edit', object: '*' },
                        { scope: 'project.custom_applications', action: 'edit', object: '*' },
                        { scope: 'project.inspections', action: 'edit', object: '*' },
                    ],
                },
                { name: 'Viewer', editable: true, permissions: [{ scope: '*', action: 'view' }], object: '*' },
                { name: 'QA', editable: true, custom: true, permissions: [{ scope: 'project.*', action: '*', object: '{"project": "staging" }' }] },
                {
                    name: 'DBA',
                    editable: true,
                    custom: true,
                    permissions: [
                        { scope: 'project.instrumentations', action: 'edit', object: '*' },
                        { scope: 'project.traces', action: 'view', object: '*' },
                        { scope: 'project.costs', action: 'view', object: '*' },
                        { scope: 'project.application', action: 'view', object: '{"application_category": "databases"}' },
                        { scope: 'project.node', action: 'view', object: '{"node_name": "db*"}' },
                    ],
                },
            ];
        },
        matrix() {
            return [
                { name: 'users:edit', roles: { Admin: '*', Editor: '', Viewer: '', QA: '', DBA: '' } },
                { name: 'project.settings:edit', roles: { Admin: '*', Editor: '', Viewer: '', QA: 'project: staging', DBA: '' } },
                { name: 'project.integrations:edit', roles: { Admin: '*', Editor: '', Viewer: '', QA: 'project: staging', DBA: '' } },
                { name: 'project.application_categories:edit', roles: { Admin: '*', Editor: '*', Viewer: '', QA: 'project: staging', DBA: '' } },
                { name: 'project.custom_applications:edit', roles: { Admin: '*', Editor: '*', Viewer: '', QA: 'project: staging', DBA: '' } },
                { name: 'project.inspections:edit', roles: { Admin: '*', Editor: '*', Viewer: '', QA: 'project: staging', DBA: '' } },
                { name: 'project.instrumentations:edit', roles: { Admin: '*', Editor: '', Viewer: '', QA: 'project: staging', DBA: '*' } },
                { name: 'project.traces:view', roles: { Admin: '*', Editor: '*', Viewer: '*', QA: 'project: staging', DBA: '*' } },
                { name: 'project.costs:view', roles: { Admin: '*', Editor: '*', Viewer: '*', QA: 'project: staging', DBA: '*' } },
                {
                    name: 'project.application:view',
                    roles: { Admin: '*', Editor: '*', Viewer: '*', QA: 'project: staging', DBA: 'application_category: databases' },
                },
                { name: 'project.node:view', roles: { Admin: '*', Editor: '*', Viewer: '*', QA: 'project: staging', DBA: 'node_name: db*' } },
            ];
        },
    },

    methods: {
        edit(role) {
            this.form.active = true;
            this.form.name = role.name;
            this.form.permissions = [...role.permissions];
        },
    },
};
</script>

<style scoped>
.form:deep(table) {
    table-layout: fixed;
    min-width: 600px;
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
