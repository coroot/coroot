<template>
    <div class="markdown">
        <template v-for="b in blocks">
            <Widget v-if="widgets[b.widget]" :w="widgets[b.widget]" class="mb-4" />
            <div v-if="b.html" v-html="b.html" />
        </template>
    </div>
</template>

<script>
import markdownIt from 'markdown-it';
// import hljs from 'highlight.js';
import Widget from '@/components/Widget.vue';

const md = new markdownIt({
    html: false,
    // highlight: (code, lang) => {
    //     if (lang && hljs.getLanguage(lang)) {
    //         try {
    //             return hljs.highlight(code, { language: lang }).value;
    //         } catch {
    //             //
    //         }
    //     }
    //     return '';
    // },
});

const widgetRE = /^WIDGET-(\d+)$/i;

export default {
    components: { Widget },
    props: {
        src: String,
        widgets: Array,
    },

    computed: {
        blocks() {
            const blocks = [];
            let buf = [];
            const renderBuf = () => {
                if (buf.length) {
                    const html = md.render(buf.join('\n'));
                    blocks.push({ html });
                    buf = [];
                }
            };
            this.src.split('\n').forEach((line) => {
                const match = line.trim().match(widgetRE);
                if (match) {
                    renderBuf();
                    blocks.push({ widget: Number(match[1]) });
                } else {
                    buf.push(line);
                }
            });
            renderBuf();
            return blocks;
        },
    },
};
</script>

<style scoped>
.markdown:deep(h1) {
    font-size: 1.5rem;
    margin-bottom: 16px;
}
.markdown:deep(h2) {
    font-size: 1.25rem;
    margin-bottom: 16px;
}
.markdown:deep(pre) {
    font-family: monospace, monospace;
    font-size: 14px;
    display: block;
    overflow-x: auto;
    padding: 12px;
    background: var(--background-color-hi);
    border-radius: 4px;
    color: var(--text-color);
    margin-bottom: 16px;
}
.markdown:deep(pre code) {
    padding: 0;
    background: unset;
    border-radius: unset;
    font-size: unset;
}
.markdown:deep(ul) {
    margin-bottom: 16px;
}
.markdown:deep(ol) {
    margin-bottom: 16px;
}
.markdown:deep(li ul) {
    margin-bottom: unset;
}
.markdown:deep(li ol) {
    margin-bottom: unset;
}
</style>
