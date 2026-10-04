<template>
    <div :class="{ compact }" @click="compact && $emit('configure')" :style="compact ? 'cursor: pointer' : ''">
        <Led :status="check.status" />
        <span>{{ compactTitle }}: </span>
        <template v-if="check.message">
            {{ check.message }}
        </template>
        <template v-else>ok</template>
        <template v-if="!compact">
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
        </template>
        <span v-else-if="hasThreshold" class="grey--text caption ml-1" @click.stop="$emit('configure')">({{ threshold }})</span>
    </div>
</template>

<script>
import Led from './Led.vue';

export default {
    props: {
        check: Object,
        compact: Boolean,
    },

    components: { Led },

    computed: {
        compactTitle() {
            if (!this.compact) return this.check.title;
            // "RUM LCP p75" → "LCP"; keep short labels readable in a chip row.
            return String(this.check.title || '')
                .replace(/^RUM\s+/i, '')
                .replace(/\s+p75$/i, '');
        },
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
.compact {
    display: inline-flex;
    align-items: center;
    flex-wrap: nowrap;
    white-space: nowrap;
    font-size: 13px;
    line-height: 1.3;
    padding: 4px 10px;
    border: 1px solid rgba(128, 128, 128, 0.35);
    border-radius: 16px;
    margin: 0 6px 6px 0;
}
</style>
