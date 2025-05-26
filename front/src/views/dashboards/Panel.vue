<template>
    <div class="panel h-100">
        <v-card outlined class="card h-100 d-flex flex-column py-2">
            <div class="d-flex flex-nowrap gap-2">
                <div class="flex-grow-1 pl-3 d-flex align-center gap-1">
                    {{ config.name }}
                    <v-tooltip v-if="config.description" bottom>
                        <template #activator="{ on }">
                            <v-icon v-on="on" small>mdi-information-outline</v-icon>
                        </template>
                        <v-card class="pa-2">
                            {{ config.description }}
                        </v-card>
                    </v-tooltip>
                    <v-progress-linear v-if="loading" color="success" height="2" indeterminate style="position: absolute; left: 0; top: 0" />
                </div>
                <div v-if="buttons" class="d-flex flex-nowrap align-center pr-1">
                    <v-btn x-small icon @click="edit"><v-icon small>mdi-pencil-outline</v-icon></v-btn>
                    <v-btn x-small icon @click="remove"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                    <v-icon size="20" class="drag">mdi-drag</v-icon>
                </div>
            </div>
            <v-alert v-if="error" color="error" text class="mt-2 rounded-0">{{ error }}</v-alert>
            <Chart v-if="data.chart" :chart="data.chart" class="flex-grow-1" />
            <div v-else class="d-flex align-center justify-center" style="height: 100%">No data</div>
        </v-card>
    </div>
</template>

<script>
import Chart from '@/components/Chart.vue';

export default {
    props: {
        config: Object,
        buttons: Boolean,
    },

    components: { Chart },

    data() {
        return {
            loading: false,
            error: '',
            data: {},
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        'config.source': { handler: 'get', deep: true },
        'config.widget': { handler: 'get', deep: true },
    },

    methods: {
        edit() {
            this.$emit('edit');
        },
        remove() {
            this.$emit('remove');
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.panelData(this.config, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.data = data || {};
            });
        },
    },
};
</script>

<style scoped>
.h-100 {
    height: 100%;
}
.name {
    text-align: center;
}
.drag {
    cursor: grab;
}
</style>
