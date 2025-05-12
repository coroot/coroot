<template>
    <v-dialog v-model="dialog" width="80%">
        <v-card class="pa-5">
            <div class="d-flex align-center">
                <div class="d-flex">
                    <v-chip label dark small :color="value.color" class="text-uppercase mr-2">{{ value.severity }}</v-chip>
                    {{ value.sum }} events
                </div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <Chart v-if="value.chart" :chart="value.chart" />
            <div class="font-weight-medium my-3">Sample</div>
            <div class="message" :class="{ multiline: value.multiline }">
                {{ value.sample }}
            </div>
            <v-btn v-if="messages" color="primary" @click="filter(value.hash)" class="mt-4"> Show messages </v-btn>
        </v-card>
    </v-dialog>
</template>

<script>
import Chart from '@/components/Chart.vue';

export default {
    props: {
        value: Object,
        messages: Boolean,
    },

    components: { Chart },

    data() {
        return {
            dialog: !!this.value,
        };
    },

    watch: {
        dialog(v) {
            !v && this.$emit('input', null);
        },
    },

    methods: {
        filter(hash) {
            this.$emit('filter', 'pattern.hash', '=', hash);
        },
    },
};
</script>

<style scoped>
.message {
    font-family: monospace, monospace;
    font-size: 14px;
    background-color: var(--background-color-hi);
    filter: brightness(var(--brightness));
    border-radius: 3px;
    max-height: 50vh;
    padding: 8px;
    overflow: auto;
}
.message.multiline {
    white-space: pre;
}
</style>
