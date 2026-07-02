<template>
    <div>
        <Led :status="check.status" />
        <span>{{ check.title }}: </span>
        <template v-if="check.message">
            {{ check.message }}
        </template>
        <template v-else>ok</template>
        <div v-if="check.details && check.details.length" class="details">
            <div v-for="(d, i) in check.details" :key="i" class="detail">
                <v-icon small color="amber darken-2" class="mr-1">mdi-alert</v-icon>{{ d }}
            </div>
        </div>
        <div class="grey--text condition">
            <span>Condition: </span>
            <span>{{ condition.head }}</span>
            <template v-if="hasThreshold">
                <a @click="$emit('configure')">{{ threshold }}</a>
                <span>{{ condition.tail }}</span>
            </template>
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
        hasThreshold() {
            return this.check.condition_format_template.includes('<threshold>');
        },
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
.details {
    margin-left: 14px;
    margin-top: 2px;
}
.details .detail {
    display: block;
    width: fit-content;
    max-width: 100%;
    font-size: 14px;
    padding: 1px 8px;
    margin: 2px 0;
    background-color: var(--background-color-hi);
    border-radius: 4px;
}
</style>
