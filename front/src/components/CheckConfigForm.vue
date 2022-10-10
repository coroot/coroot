<template>
    <v-simple-table>
        <thead>
        <tr>
            <th>Level</th>
            <th>Condition</th>
        </tr>
        </thead>
        <tbody v-if="form">
        <tr>
            <td>Override for the <var>{{$api.appId(this.appId).name}}</var> app</td>
            <td>
                <div v-if="form.application_threshold !== null" class="d-flex align-center">
                    <div class="flex-grow-1 capfirst py-3">
                        {{condition.head}}
                        <!-- eslint-disable-next-line vue/no-mutating-props -->
                        <v-text-field outlined hide-details v-model="form.application_threshold" :rules="[$validators.isFloat]" class="input" />
                        {{unit}} {{condition.tail}}
                    </div>
                    <!-- eslint-disable-next-line vue/no-mutating-props -->
                    <v-btn small icon @click="form.application_threshold = null"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                </div>
                <div v-else class="grey--text">
                    The project-level override &darr; is used. <a @click="override('application')">Override</a>
                </div>
            </td>
        </tr>
        <tr>
            <td>Project-level override</td>
            <td>
                <div v-if="form.project_threshold !== null" class="d-flex align-center">
                    <div class="flex-grow-1 capfirst py-3">
                        {{condition.head}}
                        <!-- eslint-disable-next-line vue/no-mutating-props -->
                        <v-text-field outlined hide-details v-model="form.project_threshold" :rules="[$validators.isFloat]" class="input" />
                        {{unit}} {{condition.tail}}
                    </div>
                    <!-- eslint-disable-next-line vue/no-mutating-props -->
                    <v-btn small icon @click="form.project_threshold = null"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
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
                    <div class="flex-grow-1 capfirst py-3">
                        {{condition.head}}
                        <v-text-field outlined hide-details disabled :value="form.global_threshold" class="input" />
                        {{unit}} {{condition.tail}}
                    </div>
                    <div style="min-width: 28px"></div>
                </div>
            </td>
        </tr>
        </tbody>
    </v-simple-table>
</template>

<script>
export default {
    props: {
        form: Object,
        check: Object,
        appId: String,
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
    },

    methods: {
        override(level) {
            switch (level) {
                case 'project':
                    // eslint-disable-next-line vue/no-mutating-props
                    this.form.project_threshold = this.form.global_threshold;
                    return;
                case 'application':
                    // eslint-disable-next-line vue/no-mutating-props
                    this.form.application_threshold = this.form.project_threshold || this.form.global_threshold;
                    return;
            }
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