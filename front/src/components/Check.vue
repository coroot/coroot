<template>
    <div>
        <Led :status="check.status" />
        <span>{{check.title}}: </span>
        <template v-if="check.message">
            {{check.message}}
        </template>
        <template v-else>ok</template>
        <div class="caption grey--text ml-4">
            <span>{{rule.head}}</span>
            <a>{{threshold}}</a>
            <span>{{rule.tail}}</span>
        </div>
    </div>
</template>

<script>
import Led from "@/components/Led";

export default {
    props: {
        check: Object,
    },

    components: {Led},

    computed: {
        rule() {
            const parts = this.check.rule_format_template.split('<threshold>', 2);
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
                case 'duration':
                    return this.$moment.duration(this.check.threshold, 's').format('s[s] S[ms]', {trim: 'all'})
            }
            return this.check.threshold;
        },
    }
}
</script>

<style scoped>

</style>