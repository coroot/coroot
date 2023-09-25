<template>
    <div v-if="items.length" class="d-flex flex-wrap align-center">
        <v-checkbox v-for="c in items" :key="c.name" v-model="c.value" :label="c.name" class="my-0 mx-2 text-no-wrap" color="green" hide-details />

        <v-btn v-if="configureTo" ref="configure" :to="configureTo" icon small class="ml-1" style="margin-top: 2px">
            <v-icon>mdi-plus</v-icon>
        </v-btn>
        <v-tooltip :activator="$refs.configure" bottom>
            configure categories
        </v-tooltip>
    </div>
</template>

<script>
const storageKey = 'application-categories';

export default {
    props: {
        categories: Array,
        configureTo: {
            type: Object,
            default: () => ({name: 'project_settings', params: {tab: 'categories'}}),
        },
    },

    data() {
        return {
            items: [],
        };
    },

    watch: {
        categories: {
            handler() {
                if (!this.categories.length) {
                    return;
                }
                const items = this.categories.map(c => ({name: c, value: false}));
                items.sort((a, b) => a.name.localeCompare(b.name));
                const saved = new Set(this.loadSelected());
                items.forEach(i => {
                    i.value = saved.has(i.name);
                })
                if (!items.some(i => i.value)) {
                    items[0].value = true;
                }
                this.items = items;
                this.emit();
            },
            immediate: true,
        },
        items: {
            handler() {
                this.emit();
                this.saveSelected();
            },
            deep: true,
        },
    },

    methods: {
        emit() {
            this.$emit('change', this.getSelected());
        },
        getSelected() {
            return this.items.filter(i => i.value).map(i => i.name);
        },
        loadSelected() {
            const projectId = this.$route.params.projectId;
            return (this.$storage.local(storageKey) || {})[projectId] || [];
        },
        saveSelected() {
            const saved = this.$storage.local(storageKey);
            const projectId = this.$route.params.projectId;
            saved[projectId] = this.getSelected();
            this.$storage.local(storageKey, saved);
        },
    },
}
</script>

<style scoped>

</style>