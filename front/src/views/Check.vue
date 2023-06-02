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
            <a @click="editing = true">{{threshold}}</a>
            <span>{{condition.tail}}</span>
        </div>

        <CheckForm :appId="appId" :check="check" v-model="editing"/>
    </div>
</template>

<script>
import Led from "../components/Led";
import CheckForm from "./CheckForm";

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: {CheckForm, Led},

    data() {
        return {
            editing: false,
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
                    return this.$format.duration(this.check.threshold * 1000, 'ms');
            }
            return this.check.threshold;
        },
    },
}
</script>

<style scoped>
</style>