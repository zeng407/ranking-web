<template>
  <p>
    {{ count }} / {{ maxLength }}
  </p>
</template>

<script>
export default {
  props: {
    words: {
      type: String|Number|null,
      required: true,
    },
    maxLength: {
      type: String|Number,
      required: true,
    }
  },
  computed: {
    count() {
      if(this.words === null) {
        return 0;
      }
      return this.words.length;
    },
  },
  methods: {
    showGuide() {
      $('#uploadGuide').modal('show');
    },
  },
};
</script>