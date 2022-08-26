import Vue from 'vue';
import { ResizeObserver as Polyfill } from '@juggle/resize-observer';

const ResizeObserver = window.ResizeObserver || Polyfill;

Vue.directive('on-resize', {
    bind: function (el, binding) {
        el._resizeObserver = new ResizeObserver(binding.value);
        el._resizeObserver.observe(el);
    },
    unbind: function (el) {
        el._resizeObserver.disconnect();
        delete el._resizeObserver;
    },
});
