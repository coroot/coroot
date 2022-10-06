<template>
    <div>
        <Led :status="check.status" />
        <span>{{check.title}}: </span>
        <template v-if="check.message">
            {{check.message}}
        </template>
        <template v-else>ok</template>
        <div class="grey--text ml-4">
            <span>Condition: </span>
            <span>{{condition.head}}</span>
            <a @click="dialog = true">{{threshold}}</a>
            <span>{{condition.tail}}</span>
        </div>

        <v-dialog v-model="dialog" max-width="800">
            <v-card class="pa-4">
                <div class="d-flex align-center font-weight-medium mb-4">
                    <template v-if="check.id === 'SLOAvailability' || check.id === 'SLOLatency'">
                        Configure the "{{ check.title }}" check
                    </template>
                    <template v-else>
                        Adjust the threshold for the "{{ check.title }}" check
                    </template>
                    <v-spacer />
                    <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <CheckFormSLOAvailability v-if="check.id === 'SLOAvailability'" :appId="appId" :check="check" :open="dialog" />
                <CheckFormSLOLatency v-else-if="check.id === 'SLOLatency'" :appId="appId" :check="check" :open="dialog" />
                <CheckFormSimple v-else :appId="appId" :check="check" :open="dialog" />
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import Led from "@/components/Led";
import CheckFormSimple from "@/components/CheckFormSimple";
import CheckFormSLOAvailability from "@/components/CheckFormSLOAvailability";
import CheckFormSLOLatency from "@/components/CheckFormSLOLatency";

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: {CheckFormSLOLatency, CheckFormSLOAvailability, CheckFormSimple, Led},

    data() {
        return {
            dialog: false,
        }
    },

    computed: {
        condition() {
            const parts = this.check.condition_format_template.split('<threshold>', 2);
            if (parts.length === 0) {
                return {head: '', tail: ''};
            }
            if (parts.length === 1) {
                return {head: parts[0], tail: ''};
            }
            return {head: parts[0], tail: parts[1]};
        },
        threshold() {
            switch (this.check.unit) {
                case 'percent':
                    return this.check.threshold + '%';
                case 'second':
                    return this.$moment.duration(this.check.threshold, 's').format('s[s] S[ms]', {trim: 'all'})
            }
            return this.check.threshold;
        },
    },
}
</script>

<style scoped>
</style>