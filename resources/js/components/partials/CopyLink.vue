<template>
  <div class="d-inline-block">
    <button @click="copy" :id="'popover-button-event'+id" type="button" :class="getClass()">
      <component :is="headingTag" class="m-0">
        <i class="fa-solid fa-copy"></i>&nbsp;{{ text }}
      </component>
    </button>
    <b-popover ref="popover" :placement="placement" :target="'popover-button-event'+id" :disabled="true">{{ afterCopyText }}</b-popover>
  </div>

</template>

<script>
export default {
  props: {
    id: {
      type: String,
      required: true
    },
    url: {
      type: String,
      required: true
    },
    text: {
      type: String,
      default: 'Share'
    },
    afterCopyText: {
      type: String,
      default: 'Copied link'
    },
    afterCopyFunction: {
      type: Function,
      default: () => { }
    },
    customClass: {
      type: String,
      default: ''
    },
    headingTag: {
      type: String,
      default: 'div'
    },
    placement: {
      type: String,
      default: 'right'
    }
  },
  methods: {
    copy() {
      this.$refs.popover.$emit('open');
      navigator.clipboard.writeText(this.url).then(() => {
        this.$refs.popover.$emit('show');
        this.afterCopyFunction();
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 1000);
      });
    },
    getClass() {
      if(this.customClass !== ''){
        return this.customClass;
      }else{
        return 'btn btn-outline-dark btn-sm';
      }
    }
  }
};
</script>
