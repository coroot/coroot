<template>
    <v-data-table v-bind="$attrs" v-on="$listeners" :items-per-page="perPage" @update:items-per-page="save">
        <template v-for="name in Object.keys($scopedSlots)" #[name]="scope">
            <slot :name="name" v-bind="scope" />
        </template>
    </v-data-table>
</template>

<script>
export default {
    inheritAttrs: false,

    props: {
        tableId: { type: String, required: true },
        itemsPerPage: { type: Number, default: 10 },
    },

    data() {
        return {
            perPage: Number(localStorage.getItem(this.tableId + '-items-per-page')) || this.itemsPerPage,
        };
    },

    methods: {
        save(value) {
            this.perPage = value;
            localStorage.setItem(this.tableId + '-items-per-page', value);
        },
    },
};
</script>
