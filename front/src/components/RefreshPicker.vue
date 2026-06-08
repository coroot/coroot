<template>
    <v-menu v-model="menu" :close-on-content-click="false" left offset-y attach=".v-app-bar">
        <template #activator="{ on, attrs }">
            <v-btn v-on="on" plain outlined height="40" class="px-2 ml-2">
                <v-icon>mdi-refresh</v-icon>
                <span v-if="!small" class="ml-1">{{ selectedRefresh === 0 ? 'Refresh' : currentLabel }}</span>
                <v-icon v-if="!small" small class="ml-1">mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }}</v-icon>
            </v-btn>
        </template>
        <v-list dense class="list">
            <v-list-item @click="manualRefresh">
                <v-list-item-icon class="mr-2"><v-icon small>mdi-refresh</v-icon></v-list-item-icon>
                <v-list-item-content>Refresh now</v-list-item-content>
            </v-list-item>
            <v-divider />
            <v-list-item
                v-for="r in refreshOptions"
                :key="r.value"
                @click="setRefresh(r.value)"
                :input-value="selectedRefresh === r.value"
            >
                <v-list-item-content>{{ r.text }}</v-list-item-content>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script>
export default {
    props: {
        small: Boolean,
    },
    data() {
        return {
            menu: false,
            refreshInterval: null,
            selectedRefresh: 0,
            refreshOptions: [
                { text: 'Off', value: 0 },
                { text: '30s', value: 30 },
                { text: '1m', value: 60 },
                { text: '5m', value: 300 },
                { text: '10m', value: 600 },
            ],
        };
    },
    computed: {
        currentLabel() {
            const opt = this.refreshOptions.find((r) => r.value === this.selectedRefresh);
            return opt ? opt.text : 'Off';
        },
    },
    mounted() {
        const saved = this.$storage.local('auto-refresh');
        this.selectedRefresh = saved || 0;
        this.startRefresh();
    },
    beforeDestroy() {
        this.stopRefresh();
    },
    methods: {
        manualRefresh() {
            this.$events.emit('refresh');
            this.menu = false;
        },
        startRefresh() {
            this.stopRefresh();
            if (this.selectedRefresh > 0) {
                this.refreshInterval = setInterval(() => {
                    this.$events.emit('refresh');
                }, this.selectedRefresh * 1000);
            }
        },
        stopRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
        },
        setRefresh(val) {
            this.selectedRefresh = val;
            this.$storage.local('auto-refresh', val);
            this.startRefresh();
            this.menu = false;
        },
    },
};
</script>

<style scoped>
.list:deep(.v-list-item) {
    min-height: 36px;
}
</style>
