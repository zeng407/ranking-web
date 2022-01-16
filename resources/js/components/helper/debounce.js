/**
 * function
 */
let debounceTimer = null;
export const debounceMixin = {
    methods: {
        debounce: function (callback, duration, onlyFirstExecution) {
            return function (...args) {
                let ctx = this;
                const delay = function () {
                    debounceTimer = null;
                    if(!onlyFirstExecution) callback.apply(ctx, args);
                };
                let executeNow = onlyFirstExecution && !debounceTimer;
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(delay, duration);
                if(executeNow) callback.apply(ctx, args);
            }
        }
    }
};
