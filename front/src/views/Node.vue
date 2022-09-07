<template>
<div>
    <h1 class="text-h5 my-5">
        Nodes / {{name}}
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <Dashboard v-if="node" :widgets="node.widgets" class="mt-3" />
</div>
</template>

<script>
import Dashboard from "@/components/Dashboard";

export default {
    props: {
        name: String,
    },

    components: {Dashboard},

    data() {
        return {
            node: null,
            loading: false,
            error: '',
        }
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        name() {
            this.node = null;
            this.get();
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getNode(this.name, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.node = data;
            });
        },
    },
}
</script>

<style scoped>

</style>
