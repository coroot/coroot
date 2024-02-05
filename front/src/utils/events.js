import Vue from 'vue';

export default new Vue({
    data() {
        return {
            events: {
                refresh: 0,
                'project-saved': 0,
                'project-deleted': 0,
            },
        };
    },

    methods: {
        watch(component, handler, ...events) {
            events.forEach((event) => {
                if (this.events[event] === undefined) {
                    console.warn('unknown event:', event);
                    return;
                }
                component.$watch(
                    () => {
                        return this.events[event];
                    },
                    () => {
                        handler();
                    },
                );
            });
        },
        emit(event) {
            if (!this.events[event] === undefined) {
                console.warn('unknown event:', event);
                return;
            }
            this.events[event]++;
        },
    },
});
