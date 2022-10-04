<template>
    <div>
        <Led :status="check.status" />
        <span>{{check.title}}: </span>
        <template v-if="check.message">
            {{check.message}}
        </template>
        <template v-else>ok</template>
        <div class="grey--text ml-4">
            <span>Condition: </span>
            <span>{{condition.head}}</span>
            <a @click="dialog = true">{{threshold}}</a>
            <span>{{condition.tail}}</span>
        </div>

        <v-dialog v-model="dialog" max-width="800">
            <v-card v-if="loading" class="pa-10">
                <v-progress-linear indeterminate />
            </v-card>
            <v-card v-else class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    Adjust the threshold for the "{{ check.title }}" check
                    <v-spacer />
                    <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <v-form v-model="valid">
                    <v-simple-table>
                        <thead>
                        <tr>
                            <th>Level</th>
                            <th>Condition</th>
                        </tr>
                        </thead>
                        <tbody v-if="config">
                        <tr>
                            <td>Override for the <var>{{$api.appId(this.appId).name}}</var> app</td>
                            <td>
                                <div v-if="config.application_threshold !== null" class="d-flex align-center">
                                    <div class="flex-grow-1 capfirst">
                                        {{condition.head}}
                                        <v-text-field outlined hide-details v-model="config.application_threshold" :rules="[$validators.isFloat]" class="input" />
                                        {{unit}} {{condition.tail}}
                                    </div>
                                    <v-btn small icon @click="config.application_threshold = null"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                                </div>
                                <div v-else class="grey--text">
                                    The project-level default &darr; is used. <a @click="override('application')">Override</a>
                                </div>
                            </td>
                        </tr>
                        <tr>
                            <td>Project-level default</td>
                            <td>
                                <div v-if="config.project_threshold !== null" class="d-flex align-center">
                                    <div class="flex-grow-1 capfirst">
                                        {{condition.head}}
                                        <v-text-field outlined hide-details v-model="config.project_threshold" :rules="[$validators.isFloat]" class="input" />
                                        {{unit}} {{condition.tail}}
                                    </div>
                                    <v-btn small icon @click="config.project_threshold = null"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                                </div>
                                <div v-else class="grey--text">
                                    The global default &darr; is used. <a @click="override('project')">Override</a>
                                </div>
                            </td>
                        </tr>
                        <tr>
                            <td>Global default</td>
                            <td>
                                <div class="cd-flex align-center">
                                    <div class="flex-grow-1 capfirst">
                                        {{condition.head}}
                                        <v-text-field outlined hide-details disabled :value="config.global_threshold" class="input" />
                                        {{unit}} {{condition.tail}}
                                    </div>
                                    <div style="min-width: 28px"></div>
                                </div>
                            </td>
                        </tr>
                        </tbody>
                    </v-simple-table>
                </v-form>
                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                    {{error}}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text>
                    {{message}}
                </v-alert>
                <v-btn block color="primary" @click="save" :disabled="!(valid && changed)" :loading="saving" class="mt-5">
                    Save
                </v-btn>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import Led from "@/components/Led";

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: {Led},

    data() {
        return {
            dialog: false,
            loading: false,
            error: '',
            message: '',
            config: null,
            form: null,
            valid: false,
            saving: false,
        }
    },

    watch: {
        dialog() {
            this.dialog && this.get();
        }
    },

    computed: {
        condition() {
            const parts = this.check.condition_format_template.split('<threshold>', 2);
            if (parts.length === 0) {
                return {head: '', tail: ''};
            }
            if (parts.length === 1) {
                return {head: parts[0], tail: ''};
            }
            return {head: parts[0], tail: parts[1]};
        },
        threshold() {
            switch (this.check.unit) {
                case 'percent':
                    return this.check.threshold + '%';
                case 'second':
                    return this.$moment.duration(this.check.threshold, 's').format('s[s] S[ms]', {trim: 'all'})
            }
            return this.check.threshold;
        },
        unit() {
            switch (this.check.unit) {
                case 'percent':
                    return '%';
                case 'second':
                    return 'seconds'
            }
            return '';
        },
        changed() {
            return !!this.config && JSON.stringify(this.form) !== JSON.stringify(this.toForm());
        },
    },

    methods: {
        toForm() {
            return {
                project_threshold: parseFloat(this.config.project_threshold),
                application_threshold: parseFloat(this.config.application_threshold),
            };
        },
        override(level) {
            switch (level) {
                case 'project':
                    this.config.project_threshold = this.config.global_threshold;
                    return;
                case 'application':
                    this.config.application_threshold = this.config.project_threshold || this.config.global_threshold;
                    return;
            }
        },
        get() {
            this.loading = true;
            this.$api.getCheckConfig(this.appId, this.check.id, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    this.config = null;
                    return;
                }
                this.config = data;
                this.form = this.toForm();
            })
        },
        save() {
            const form = this.toForm();
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveCheckConfig(this.appId, this.check.id, form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                    this.dialog = false;
                }, 1000);
            })
        },
    }
}
</script>

<style scoped>
td {
    font-size: 1rem !important;
}
.capfirst:first-letter {
    text-transform: uppercase;
}
.input {
    display: inline-flex;
    max-width: 8ch;
}
.input >>> .v-input__slot {
    min-height: initial !important;
    height: 1.5rem !important;
    padding: 0 8px !important;
}
</style>