<template>
    <div class="code">
        <v-btn icon small dark class="copy" @click="copy" :disabled="disabled">
            <v-icon small>{{ icon }}</v-icon>
        </v-btn>
        <div ref="body">
            <slot></slot>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        disabled: Boolean,
    },

    data() {
        return {
            copied: false,
        };
    },

    computed: {
        icon() {
            return this.copied ? 'mdi-check' : 'mdi-content-copy';
        },
    },

    methods: {
        copy() {
            navigator.clipboard.writeText(this.$refs.body.innerText.trim());
            this.copied = true;
            setTimeout(() => {
                this.copied = false;
            }, 1000);
        },
    },
};
</script>

<style scoped>
.code {
    position: relative;
    margin-bottom: 12px;
}
.code:deep(pre) {
    font-family: monospace, monospace;
    font-size: 14px;
    display: block;
    overflow-x: auto;
    padding: 20px 20px 0 20px;
    background: #282a36;
    border-radius: 4px;
    color: var(--text-dark);
}
.copy {
    position: absolute;
    top: 10px;
    right: 10px;
}
</style>
