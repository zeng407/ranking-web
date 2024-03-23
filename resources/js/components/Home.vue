<script>
export default {
  name: 'home',
  mounted() {
    window.addEventListener('scroll', this.handleScroll);
    if (this.sortBy === 'hot') {
      if (this.range === 'all') {
        this.timeRangeText = this.$t('All Time');
      } else if (this.range === 'week') {
        this.timeRangeText = this.$t('This Week');
      } else if (this.range === 'day') {
        this.timeRangeText = this.$t('Today');
      } else if (this.range === 'month') {
        this.timeRangeText = this.$t('This Month');
      } else if (this.range === 'year') {
        this.timeRangeText = this.$t('This Year');
      }
    }

  },
  props: {
    playGameRoute: String,
    gameRankRoute: String,
    sortBy: {
      type: String,
      default: 'hot'
    },
    keyword: String,
    range: {
      type: String,
      default: 'month'
    },
    getTagsOptionsEndpoint: String
  },
  data: function () {
    return {
      filters: {
        keyword: this.keyword,
        sort_by: this.sortBy,
        range: this.range
      },
      timeRangeText: '',
    }
  },
  watch: {
    'filters.sort_by': function (value) {
      this.loadData();
    }
  },
  methods: {
    loadData: function () {
      let query = {
        k: this.filters.keyword,
        sort_by: this.filters.sort_by,
        range: this.filters.range
      };
      // Remove null values from the query
      query = Object.fromEntries(Object.entries(query).filter(([_, v]) => v !== null && v !== ''));
      window.location.href = '?' + new URLSearchParams(query).toString();
    },
    addTag: function (tag) {
      this.filters.keyword = tag;
      this.loadData();
    },
    share: function (url, title, description, event) {
      if (navigator.share) {
        navigator.share({
          title: title,
          text: description,
          url: url,
        }).catch(console.error);
      }else{
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 1000);
      }
    },
    search: function () {
      this.loadData();
    },
    clickTimeRange: function (event, value) {
      this.filters.range = value;
      this.timeRangeText = event.target.text;
      this.loadData();
    }
  }
}
</script>