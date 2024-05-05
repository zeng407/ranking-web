

import {ValidationProvider, ValidationObserver, extend, localize, configure} from 'vee-validate';
import en from 'vee-validate/dist/locale/en.json';
import zh_TW from 'vee-validate/dist/locale/zh_TW.json';
configure({
    defaultMessage: (field, values) => {
      // override the field name.
      values._field_ = i18n.t(`fields.${field}`);
  
      return i18n.t(`validation.${values._rule_}`, values);
    }
});
localize({en, zh_TW});

const getUserLocale = () => {
    const locale = document.documentElement.lang || 'en';
    //replace - to _ for vee-validate
    return locale.replace('-', '_');
};
const userLocale = getUserLocale();
localize(userLocale);

/**
 * register validation rule
 */
import {required, min_value} from 'vee-validate/dist/rules';
import Vue from 'vue';
Vue.component('ValidationProvider', ValidationProvider);
Vue.component('ValidationObserver', ValidationObserver);
extend('required', required);
extend('min_value', min_value);