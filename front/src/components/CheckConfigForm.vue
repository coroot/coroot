<template>
    <v-simple-table>
        <thead>
            <tr>
                <th>Level</th>
                <th>Condition</th>
            </tr>
        </thead>
        <tbody v-if="form">
            <tr v-if="form.configs.length === 3">
                <td>
                    Override for the <var>{{ $utils.appId(this.appId).name }}</var> app
                </td>
                <td>
                    <div v-if="form.configs[2] !== null" class="d-flex align-center">
                        <div class="flex-grow-1 capfirst py-3">
                            {{ condition.head }}
                            <!-- eslint-disable vue/no-mutating-props -->
                            <v-text-field
                                outlined
                                hide-details
                                v-model.number="form.configs[2].threshold"
                                :rules="[$validators.isFloat]"
                                class="input"
                            />
                            <!-- eslint-enable vue/no-mutating-props -->
                            {{ unit }} {{ condition.tail }}
                        </div>
                        <!-- eslint-disable-next-line vue/no-mutating-props -->
                        <v-btn small icon @click="override(2, true)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                    </div>
                    <div v-else class="grey--text">The project-level override &darr; is used. <a @click="override(2)">Override</a></div>
                </td>
            </tr>
            <tr>
                <td>Project-level override</td>
                <td>
                    <div v-if="form.configs[1] !== null" class="d-flex align-center">
                        <div class="flex-grow-1 capfirst py-3">
                            {{ condition.head }}
                            <!-- eslint-disable vue/no-mutating-props -->
                            <v-text-field
                                outlined
                                hide-details
                                v-model.number="form.configs[1].threshold"
                                :rules="[$validators.isFloat]"
                                class="input"
                            />
                            <!-- eslint-enable vue/no-mutating-props -->
                            {{ unit }} {{ condition.tail }}
                        </div>
                        <!-- eslint-disable-next-line vue/no-mutating-props -->
                        <v-btn small icon @click="override(1, true)"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                    </div>
                    <div v-else class="grey--text">The global default &darr; is used. <a @click="override(1)">Override</a></div>
                </td>
            </tr>
            <tr>
                <td>Global default</td>
                <td>
                    <div class="cd-flex align-center">
                        <div class="flex-grow-1 capfirst py-3">
                            {{ condition.head }}
                            <v-text-field outlined hide-details disabled :value="form.configs[0].threshold" class="input" />
                            {{ unit }} {{ condition.tail }}
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
                return { head: '', tail: '' };
            }
            if (parts.length === 1) {
                return { head: parts[0], tail: '' };
            }
            return { head: parts[0], tail: parts[1] };
        },
        unit() {
            switch (this.check.unit) {
                case 'percent':
                    return '%';
                case 'second':
                    return 'seconds';
                case 'seconds/second':
                    return 'seconds/second';
            }
            return '';
        },
    },

    methods: {
        override(level, drop) {
            if (drop) {
                // eslint-disable-next-line vue/no-mutating-props
                this.$set(this.form.configs, level, null);
                return;
            }

            let th = null;
            for (let l = level - 1; l >= 0; l--) {
                if (this.form.configs[l]) {
                    th = this.form.configs[l].threshold;
                }
            }
            if (this.form.configs[level] === null) {
                // eslint-disable-next-line vue/no-mutating-props
                this.$set(this.form.configs, level, {});
            }
            // eslint-disable-next-line vue/no-mutating-props
            this.form.configs[level].threshold = th;
        },
    },
};
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
