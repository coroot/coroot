<template>
    <div ref="code" class="code">
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
            const textarea = document.createElement('textarea');
            this.$refs.code.appendChild(textarea);
            textarea.value = this.$refs.body.innerText.trim();
            textarea.focus();
            textarea.select();
            try {
                document.execCommand('copy');
                this.copied = true;
                setTimeout(() => {
                    this.copied = false;
                }, 3000);
            } finally {
                this.$refs.code.removeChild(textarea);
            }
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
