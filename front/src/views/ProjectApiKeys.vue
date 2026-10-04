<template>
    <div style="max-width: 800px">
        <h2 class="text-h5 mt-10 mb-5">API keys</h2>
        <p>
            API keys authorize Coroot's agents and applications to write telemetry. Use type <b>rum</b> for browser Real User Monitoring — those keys
            require allowed origins and only accept browser (webjs) OTLP traffic.
        </p>
        <v-simple-table dense>
            <thead>
                <tr>
                    <th>Description</th>
                    <th>Type</th>
                    <th>Key</th>
                    <th>Allowed origins</th>
                    <th style="width: 100px">Actions</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="k in keys" :key="k.key || k.description">
                    <td>{{ k.description }}</td>
                    <td>{{ k.type || 'default' }}</td>
                    <td>
                        <template v-if="k.key"> {{ k.key }} <CopyButton :text="k.key" /> </template>
                        <template v-else>
                            <span class="grey--text">Only project Admins can access API keys.</span>
                        </template>
                    </td>
                    <td>
                        <span v-if="k.allowed_origins && k.allowed_origins.length">{{ k.allowed_origins.join(', ') }}</span>
                        <span v-else class="grey--text">—</span>
                    </td>
                    <td>
                        <v-btn icon small @click="open('edit', k)" :disabled="!editable"><v-icon small>mdi-pencil</v-icon></v-btn>
                        <v-btn icon small @click="open('delete', k)" :disabled="!editable"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>

        <v-btn color="primary" class="mt-4 mr-2" small @click="open('generate', {})" :disabled="!editable">Generate API key</v-btn>
        <v-btn color="primary" class="mt-4" outlined small @click="open('generate', { type: 'rum' })" :disabled="!editable">Generate RUM key</v-btn>

        <v-dialog v-model="dialog" max-width="600">
            <v-card v-if="loading" class="pa-10">
                <v-progress-linear indeterminate />
            </v-card>
            <v-card v-else class="pa-4">
                <v-form @submit.prevent="post">
                    <div class="d-flex align-center font-weight-bold mb-4">
                        <div v-if="form.action === 'generate'">Generate {{ form.type === 'rum' ? 'RUM' : 'API' }} key</div>
                        <div v-else-if="form.action === 'delete'">Delete API key</div>
                        <div v-else>Edit API key</div>
                        <v-spacer />
                        <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
                    </div>
                    <p v-if="form.action === 'delete'">
                        Deleting the API key can result in some agents or applications using it no longer being able to write telemetry data to this
                        project.
                    </p>
                    <div class="subtitle-1">Description</div>
                    <v-text-field
                        ref="descriptionField"
                        v-model="form.description"
                        outlined
                        dense
                        autofocus
                        :readonly="form.action === 'delete'"
                    ></v-text-field>
                    <template v-if="form.action !== 'delete'">
                        <div class="subtitle-1">Type</div>
                        <v-select
                            v-model="form.type"
                            :items="[
                                { value: '', text: 'default (agents / backend OTLP)' },
                                { value: 'rum', text: 'rum (browser RUM)' },
                            ]"
                            outlined
                            dense
                            :disabled="form.action === 'edit'"
                            :menu-props="{ offsetY: true }"
                        />
                        <template v-if="form.type === 'rum'">
                            <div class="subtitle-1">Allowed origins (one per line)</div>
                            <v-textarea
                                v-model="originsText"
                                outlined
                                dense
                                rows="3"
                                placeholder="https://shop.example.com&#10;https://www.example.com"
                                hint="Browser Origin must match exactly (or use *)."
                                persistent-hint
                            />
                        </template>
                    </template>
                    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                        {{ error }}
                    </v-alert>
                    <div class="d-flex align-center">
                        <v-spacer />
                        <v-btn
                            v-if="form.action === 'generate'"
                            type="submit"
                            color="primary"
                            :disabled="!form.description || (form.type === 'rum' && !originsText.trim())"
                            :loading="loading"
                        >
                            Generate
                        </v-btn>
                        <v-btn v-else-if="form.action === 'delete'" type="submit" color="error" :loading="loading" autofocus> Delete </v-btn>
                        <v-btn v-else type="submit" color="primary" :disabled="!form.description" :loading="loading"> Save </v-btn>
                    </div>
                </v-form>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import CopyButton from '@/components/CopyButton.vue';

export default {
    components: { CopyButton },

    data() {
        return {
            loading: false,
            error: '',
            editable: false,
            keys: [],
            dialog: false,
            originsText: '',
            form: {
                action: '',
                key: '',
                description: '',
                type: '',
                allowed_origins: [],
            },
        };
    },

    mounted() {
        this.get();
    },

    methods: {
        open(action, key) {
            this.dialog = true;
            this.error = '';
            this.form.action = action;
            this.form.key = key.key || '';
            this.form.description = key.description || '';
            this.form.type = key.type || '';
            this.form.allowed_origins = key.allowed_origins || [];
            this.originsText = (key.allowed_origins || []).join('\n');
        },
        get() {
            this.error = '';
            this.loading = true;
            this.$api.apiKeys(null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.editable = data.editable;
                this.keys = data.keys || [];
            });
        },
        post() {
            this.error = '';
            this.loading = true;
            if (this.form.type === 'rum') {
                this.form.allowed_origins = this.originsText
                    .split('\n')
                    .map((s) => s.trim())
                    .filter(Boolean);
            } else {
                this.form.allowed_origins = [];
            }
            this.$api.apiKeys(this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                setTimeout(() => {
                    this.dialog = false;
                }, 500);
                this.get();
            });
        },
    },
};
</script>

<style scoped></style>
