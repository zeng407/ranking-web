
<script>
export default {
  props: {
    words: {
      type: Array,
      required: true,
    },
  },
  data: function () {
    return {
      word: '',
    }
  },
  mounted() {
    this.getRandomWord();
  },
  methods: {
    getRandomWord() {
      let randomIndex = Math.floor(Math.random() * this.words.length);
      this.word = this.words[randomIndex];
    },
  },
};
</script>