<template>
    <v-select
        :items="items"
        :value="value"
        hide-details
        dense
        outlined
        :menu-props="{ offsetY: true }"
        class="select"
        @change="change"
        :style="style"
    />
</template>

<script>
export default {
    props: {
        value: Number,
        items: Array,
    },

    computed: {
        style() {
            return {
                minWidth: (this.value + '').length + 5 + 'ch',
            };
        },
    },

    methods: {
        change(v) {
            this.$emit('input', v);
        },
    },
};
</script>

<style scoped>
.select {
    display: inline-block;
    font-size: inherit;
}
.select:deep(.v-input__control) {
    min-height: unset !important;
}
.select:deep(.v-input__slot) {
    min-height: unset !important;
    margin: 0 !important;
    padding: 0 !important;
}
.select:deep(.v-input__append-inner) {
    margin-top: 0 !important;
    padding: 0 !important;
}
.select:deep(.v-select__selections) {
    padding: 0 !important;
}
.select:deep(.v-select__selection--comma) {
    margin: 0 !important;
    padding: 0 0 0 4px !important;
}
.select:deep(input) {
    display: none;
}
* >>> .v-list-item {
    min-height: 32px !important;
    padding: 0 8px !important;
}
</style>
