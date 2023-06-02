<template>
    <div>
        <div v-if="opened">
            <Chart :chart="opened.instances" />
            <div :class="`sample details ${opened.multiline ? 'multiline' : ''} ma-3 pa-3`" v-html="opened.sample" />
        </div>

        <div v-else>
            <div v-if="title" class="text-center mb-2 title" v-html="title"></div>

            <template v-for="p in patterns">
                <div v-if="!featuredOnly || p.featured" class="pattern" @click.stop="openDetails(p)">
                    <div class="sample preview" v-html="p.sample" />
                    <div class="line">
                        <v-sparkline :value="p.sum.map((v) => v === null ? 0 : v)" smooth height="30" fill :color="color(p.color)" padding="4" />
                    </div>
                    <div class="percent" v-html="(p.percentage < 1 ? '<span>&lt;</span>1' : p.percentage) + '%'" />
                </div>
            </template>

            <v-dialog v-model="details" width="80%">
                <v-card v-if="pattern" tile class="pa-3">
                    <Chart :chart="pattern.instances" />
                    <div :class="`sample details ${pattern.multiline ? 'multiline' : ''} ma-3 pa-3`" v-html="pattern.sample" />
                    <v-btn icon x-small absolute top right @click="details = false"><v-icon>mdi-close</v-icon></v-btn>
                </v-card>
            </v-dialog>
        </div>
    </div>
</template>

<script>
import Chart from './Chart';
import { palette } from '../utils/colors';

export default {
    props: {
        title: String,
        openWithSubstr: String,
        patterns: Array,
        featuredOnly: Boolean,
    },
    components: {Chart},

    data() {
        return {
            pattern: null,
            details: false,
        };
    },

    mounted() {
        this.$nextTick(() => {
            Array.from(this.$el.getElementsByClassName('sample preview')).forEach((el) => {
                const marks = el.getElementsByTagName('mark');
                if (marks.length === 0) {
                    return;
                }
                const mark = marks[0];
                el.scrollTop = mark.offsetTop - el.offsetTop;
            });
        });
    },
    computed: {
        opened() {
            if (!this.openWithSubstr) {
                return null;
            }
            return this.patterns.find((p) => p.sample.includes(this.openWithSubstr));
        },
    },

    methods: {
        openDetails(pattern) {
            this.pattern = pattern;
            this.details = true;
        },
        color(name) {
            return palette.get(name, 0);
        },
    },
};
</script>

<style scoped>
.title {
    font-size: 14px !important;
    font-weight: normal !important;
}
.pattern {
    display: flex;
    align-items: flex-end;
    margin-bottom: 8px;
    cursor: pointer;
    background-color: #EEEEEE;
    padding: 4px 8px;
    border-radius: 3px;
}
.pattern:hover {
    background-color: #E0E0E0;
}
.sample {
    font-size: 0.8rem;
}
.sample.preview {
    flex-grow: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-height: 4.5rem;
}
.sample.details {
    background-color: #EEEEEE;
    overflow: auto;
    border-radius: 3px;
}
.sample.details.multiline {
    white-space: pre;
    max-height: 50vh;
}
.sample >>> mark {
    background-color: unset;
    color: black;
    font-weight: bold;
}
.line {
    flex-grow: 0;
    flex-basis: 30%;
    max-width: 30%;
    flex-shrink: 0;
}
.percent {
    flex-grow: 0;
    flex-basis: 2rem;
    max-width: 2rem;
    flex-shrink: 0;
    font-size: 0.75rem;
    text-align: right;
}
.percent >>> span {
    font-size: 0.65rem;
}
</style>
