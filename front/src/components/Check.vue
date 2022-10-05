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

        <CheckFormAvailability v-if="check.id === 'SLOAvailability'" v-model="dialog" :appId="appId" :check="check" />
        <CheckFormSimple v-else v-model="dialog" :appId="appId" :check="check" />
    </div>
</template>

<script>
import Led from "@/components/Led";
import CheckFormSimple from "@/components/CheckFormSimple";
import CheckFormAvailability from "@/components/CheckFormAvailability";

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: {CheckFormAvailability, CheckFormSimple, Led},

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