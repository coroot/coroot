<template>
    <div>
        <h2 class="text-h6 mt-6 mb-3">Discovery status</h2>
        <div class="d-flex align-center"><Led :status="status.led" /> {{ status.text }}</div>
        <div v-if="!configured && detected" class="mt-1" style="max-width: 800px">
            This cluster runs on {{ provider }}, but the integration is not configured: its managed databases are not discovered.
        </div>
        <div v-for="e in errors" class="error--text mt-1" style="max-width: 800px">• {{ e }}</div>

        <h2 class="text-h6 mt-6 mb-3">Discovered instances</h2>
        <v-data-table
            :items="instances"
            sort-by="application_id"
            must-sort
            dense
            class="instances"
            mobile-breakpoint="0"
            :items-per-page="20"
            no-data-text="No instances found"
            :headers="[
                { value: 'application_id', text: 'Application', align: 'start' },
                { value: 'name', text: 'Instance', align: 'start' },
                { value: 'status', text: 'Status', align: 'start' },
                { value: 'engine', text: 'Engine', align: 'start' },
                { value: 'engine_version', text: 'Version', align: 'start' },
                { value: 'instance_type', text: 'Instance type', align: 'start' },
                { value: 'availability_zone', text: 'AZ', align: 'start' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application_id="{ item }">
                <router-link :to="{ name: 'overview', params: { view: 'applications', id: item.application_id } }" class="text-no-wrap">
                    {{ $utils.appId(item.application_id).name }}
                </router-link>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import Led from './Led.vue';

export default {
    components: { Led },

    props: {
        provider: String,
        configured: Boolean,
        detected: Boolean,
        errors: Array,
        instances: Array,
        error: String,
    },

    computed: {
        status() {
            if (!this.configured) {
                return { led: this.detected ? 'warning' : 'unknown', text: 'not configured' };
            }
            if (this.errors.length) {
                return { led: 'critical', text: `${this.errors.length} ${this.errors.length === 1 ? 'error' : 'errors'}` };
            }
            if (this.error) {
                return { led: 'unknown', text: 'unknown' };
            }
            return { led: 'ok', text: 'OK' };
        },
    },
};
</script>
