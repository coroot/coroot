<template>
    <div>
        <Led :status="check.status" />
        <span>{{ check.title }}: </span>
        <template v-if="check.message">
            {{ check.message }}
        </template>
        <template v-else>ok</template>
        <div class="grey--text condition">
            <span>Condition: </span>
            <span>{{ condition.head }}</span>
            <a @click="$emit('configure')">{{ threshold }}</a>
            <span>{{ condition.tail }}</span>
        </div>
    </div>
</template>

<script>
import Led from './Led.vue';

export default {
    props: {
        check: Object,
    },

    components: { Led },

    computed: {
        condition() {
            const parts = this.check.condition_format_template.split('<threshold>', 2);
            if (parts.length === 0) {
                return { head: '', tail: '' };
            }
            if (parts.length === 1) {
                return { head: parts[0], tail: '' };
            }
            return { head: parts[0], tail: parts[1] };
        },
        threshold() {
            switch (this.check.unit) {
                case 'percent':
                    return this.check.threshold + '%';
                case 'second':
                    return this.$format.duration(this.check.threshold * 1000, 'ms');
                case 'seconds/second':
                    return this.check.threshold + ' seconds/second';
            }
            return this.check.threshold;
        },
    },
};
</script>

<style scoped>
.condition {
    margin-left: 14px;
}
</style>
