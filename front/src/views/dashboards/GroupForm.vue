<template>
    <v-dialog v-model="dialog" max-width="600">
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                <template v-if="form.action === 'delete'"> Delete panel group </template>
                <template v-else> Edit panel group </template>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form v-model="valid" :disabled="form.action === 'delete'">
                <div class="subtitle-1">Name</div>
                <v-text-field v-model="form.name" :rules="[$validators.notEmpty]" outlined dense />
            </v-form>
            <div class="d-flex mt-3 gap-1">
                <v-spacer />
                <v-btn v-if="form.action === 'delete'" color="error" @click="apply()">Delete</v-btn>
                <v-btn v-else color="primary" @click="apply()" :disabled="!valid">Apply</v-btn>
                <v-btn color="primary" @click="dialog = false" outlined>Cancel</v-btn>
            </div>
        </v-card>
    </v-dialog>
</template>

<script>
export default {
    props: {
        value: Object,
    },

    data() {
        return {
            dialog: !!this.value,
            form: JSON.parse(JSON.stringify(this.value)),
            valid: false,
        };
    },

    watch: {
        dialog(v) {
            !v && this.$emit('input', null);
        },
    },

    methods: {
        apply() {
            this.$emit(this.form.action, this.form.id, this.form.name);
            this.dialog = false;
        },
    },
};
</script>
<style scoped></style>
