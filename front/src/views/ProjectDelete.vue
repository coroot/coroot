<template>
    <div style="max-width: 800px">
        <div class="d-block d-md-flex align-center">
            <div class="flex-grow-1">
                <div><b>Delete this project</b></div>
                <div>Once you delete a project, there is no going back. Please be certain.</div>
            </div>
            <div>
                <v-btn block @click="dialog = true" :disabled="readonly" color="red" outlined>Delete this project</v-btn>
            </div>
        </div>
        <v-dialog v-model="dialog" max-width="600">
            <v-card v-if="loading" class="pa-10">
                <v-progress-linear indeterminate />
            </v-card>
            <v-card v-else class="pa-4">
                <div class="d-flex align-center font-weight-bold mb-4">
                    Are you absolutely sure?
                    <v-spacer />
                    <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <p>
                    This action cannot be undone. This will permanently delete the <b>{{ name }}</b> project.
                </p>
                <p>
                    Please type <b>{{ name }}</b> to confirm
                </p>
                <v-text-field v-model="confirmation" outlined dense></v-text-field>
                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                    {{ error }}
                </v-alert>
                <v-btn block color="red" outlined :disabled="confirmation !== name" @click="del">
                    <template v-if="$vuetify.breakpoint.mdAndUp"> I understand the consequences, delete this project </template>
                    <template v-else> Delete this project </template>
                </v-btn>
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
            readonly: false,
            name: '',
            dialog: false,
            loading: false,
            confirmation: '',
            error: '',
        };
    },

    mounted() {
        this.get();
    },

    watch: {
        dialog(v) {
            this.confirmation = '';
            v && this.get();
        },
    },

    methods: {
        get() {
            this.error = '';
            this.loading = true;
            this.$api.getProject(this.projectId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.readonly = data.readonly;
                this.name = data.name;
            });
        },
        del() {
            this.error = '';
            this.$api.delProject(this.projectId, (data, error) => {
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('projects');
                this.$router.push({ name: 'index' });
            });
        },
    },
};
</script>

<style scoped></style>
