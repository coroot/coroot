<template>
    <v-dialog v-model="dialog" max-width="600">
        <v-card class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-4">
                API keys of {{ user.email }}
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>{{ error }}</v-alert>
            <v-simple-table v-if="items.length" dense class="mb-3">
                <tbody>
                    <tr v-for="k in items">
                        <td>{{ k.description }}</td>
                        <td class="text-right">
                            <v-btn v-if="!locked" small icon @click="del(k)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                        </td>
                    </tr>
                </tbody>
            </v-simple-table>
            <div v-else class="grey--text mb-3">No API keys yet.</div>
            <div v-if="created" class="mb-3">
                <div class="font-weight-medium mb-1 warning--text">
                    <v-icon small color="warning" class="mr-1">mdi-alert-outline</v-icon>
                    New API key: copy it now, it won't be shown again
                </div>
                <v-text-field outlined dense hide-details readonly :value="created" @focus="$event.target.select()">
                    <template #append><CopyButton :text="created" /></template>
                </v-text-field>
                <div class="caption mt-1">Pass it as <code>Authorization: Bearer &lt;key&gt;</code> to the MCP endpoint or the API.</div>
            </div>
            <div v-if="locked" class="grey--text">This service account is defined in the config file, so its keys are managed there.</div>
            <div v-else class="d-flex align-center">
                <v-text-field outlined dense hide-details v-model="description" placeholder="description (required)" class="mr-2" />
                <v-btn color="primary" :disabled="!description.trim()" :loading="loading" @click="create">Create key</v-btn>
            </div>
        </v-card>
    </v-dialog>
</template>

<script>
import CopyButton from '@/components/CopyButton.vue';

export default {
    components: { CopyButton },

    props: {
        value: Boolean,
        user: Object, // {id, email}
        locked: Boolean, // keys are managed in the config file
    },

    data() {
        return {
            dialog: this.value,
            items: [],
            description: '',
            created: '',
            error: '',
            loading: false,
        };
    },

    watch: {
        value(v) {
            this.dialog = v;
        },
        dialog(v) {
            this.$emit('input', v);
            if (v) {
                this.items = [];
                this.description = '';
                this.created = '';
                this.error = '';
                this.get();
            }
        },
    },

    methods: {
        get() {
            this.$api.userApiKeys(this.user.id, null, (data, error) => {
                if (error) {
                    this.error = error;
                    return;
                }
                this.items = data || [];
            });
        },
        create() {
            this.loading = true;
            this.error = '';
            this.$api.userApiKeys(this.user.id, { action: 'create', description: this.description }, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.created = data.key;
                this.description = '';
                this.get();
            });
        },
        del(k) {
            this.$api.userApiKeys(this.user.id, { action: 'delete', id: k.id }, (data, error) => {
                if (error) {
                    this.error = error;
                    return;
                }
                this.created = '';
                this.get();
            });
        },
    },
};
</script>

<style scoped></style>
