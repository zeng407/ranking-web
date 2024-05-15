<script>
import masonry from 'masonry-layout';

export default {
  name: 'home',
  mounted() {
    this.registerScrollEvent();
    this.initSorter();
    this.initGoogleAds();
    this.initMasonry();
  },
  props: {
    indexPostsEndpoint: {
      type: String,
      required: true
    },
    showGameEndpoint: {
      type: String,
      required: true
    },
    showRankEndpoint: {
      type: String,
      required: true
    },
    sortBy: {
      type: String,
      required: true
    },
    keyword: {
      type: String,
      required: true
    },
    range: {
      type: String,
      required: true
    },
    currentPage: {
      type: Number,
      required: true
    },
  },
  data: function () {
    return {
      filters: {
        keyword: this.keyword,
        sort_by: this.sortBy,
        range: this.range,
        page: this.currentPage
      },
      timeRangeText: '',
      errorImages: [],
      isLoadingMorePosts: false,
      delayLoading: false,
      readyQueueForLoadPosts: false,
      posts: [],
      last_page: null,
      lastScrollPosition: 0,
      showReturnUpButton: false,
    }
  },
  watch: {
    'filters.sort_by': function (value) {
      this.search();
    }
  },
  computed: {
    isFetchAllPosts() {
      return this.last_page !== null && this.filters.page >= this.last_page;
    },
  },
  methods: {
    loadPosts(page) {
      // mutex lock to prevent multiple requests
      if(this.isLoadingMorePosts || this.isFetchAllPosts) {
        return;
      }
      this.isLoadingMorePosts = true;
      this.delayLoading = true;
      let params = {
        page: page,
        sort_by: this.filters.sort_by,
        range: this.filters.range,
        k: this.filters.keyword
      };
      //normalize query
      params = Object.fromEntries(Object.entries(params).filter(([_, v]) => v !== null && v !== ''));
      // if sort_by is new, we don't need to pass the range
      if (this.filters.sort_by === 'new') {
        delete params.range;
      }
      axios.get(this.indexPostsEndpoint, {
        params: params
      }).then(response => {
        this.resetLoading();
        this.filters.page = response.data.current_page;
        this.last_page = response.data.last_page;
        this.posts = this.posts.concat(response.data.data);
        //update posts
        Vue.set(this, 'posts', this.posts);

        this.initMasonry();
      }).catch(error => {
        this.resetLoading();
        console.error(error);
      });
    },
    resetLoading() {
      this.isLoadingMorePosts = false;
      setTimeout(() => {
        if(this.readyQueueForLoadPosts){
          this.readyQueueForLoadPosts = false;
          if(this.isScrollAtBottom()){
            this.loadPosts(this.filters.page+1);
          }
        }else{
          this.delayLoading = false;
        }
        }, 3000);
    },
    search() {
      let query = {
        k: this.filters.keyword,
        sort_by: this.filters.sort_by,
        range: this.filters.range
      };
      // Remove null values from the query
      query = Object.fromEntries(Object.entries(query).filter(([_, v]) => v !== null && v !== ''));
      // if sort_by is new, we don't need to pass the range
      if (this.filters.sort_by === 'new') {
        delete query.range;
      }
      window.location.href = '?' + new URLSearchParams(query).toString();
    },
    addTag(tag) {
      this.filters.keyword = tag;
      this.search();
    },
    share(url, id) {
      url = url + '?utm_medium=share_game';
      if (navigator.share) {
        navigator.share({
          url: url
        }).catch(console.error);
      } else {
        this.$refs['popover' + id].$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    },
    handleChildShare(popover, url, id) {
      url = url + '?utm_medium=share_game';
      if (navigator.share) {
        navigator.share({
          url: url
        }).catch(console.error);
      } else {
        popover.$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    },
    clickTimeRange(event, value) {
      this.filters.range = value;
      this.timeRangeText = event.target.text;
      this.search();
    },
    onImageError(replaceUrl, event) {
      if (this.errorImages.includes(replaceUrl)) {
        return;
      }

      if (replaceUrl !== null) {
        event.target.src = replaceUrl;
      }
      this.errorImages.push(replaceUrl);
    },
    initMasonry() {
      new masonry('.grid', {
        itemSelector: '.grid-item',
        columnWidth: '.grid-sizer',
        gutter: '.gutter-sizer',
        percentPosition: true,
      });
    },
    handleScroll(event) {
      if (this.isScrollAtBottom()) {
        this.triggerLoadPosts();
      }
      this.recordLastScrollPosition();
    },
    isScrollAtBottom() {
      let buffer = 80;
      return (window.innerHeight + window.scrollY) >= document.body.offsetHeight - buffer;
    },
    recordLastScrollPosition() {
      const currentScrollPosition = window.scrollY;
      if(this.recording == null){
        this.recording = setTimeout(() => {
          this.lastScrollPosition = currentScrollPosition;
          this.recording = null;
        }, 100);
      }

      if(this.isScrollingUp()) {
        this.showReturnUpButton = true;
      }else{
        this.showReturnUpButton = false;
      }      
    },
    scrollToTop() {
      window.scrollTo({
        top: 0,
        behavior: 'smooth'
      });
    },
    isScrollingUp() {
      return window.scrollY < this.lastScrollPosition && window.scrollY > 625;
    },
    initGoogleAds() {
      if (window.adsbygoogle) {
        setTimeout(() => {
          if (window.adsbygoogle) {
            if ($('#google-ad-1')) {
              $('#google-ad-1').addClass('d-flex justify-content-center');
            }
          }
        }, 1000);
      }
      //handle event when google-ads is loaded
      window.addEventListener('ad-loaded', () => {
        if (window.adsbygoogle) {
          this.initMasonry();
        }
      });
    },
    initSorter() {
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
    registerScrollEvent() {
      window.addEventListener('scroll', this.handleScroll);
    },
    triggerLoadPosts() {
      if(this.isLoadingMorePosts || this.readyQueueForLoadPosts || this.isFetchAllPosts) {
        console.log('suspend loading more posts 1');
        return;
      }

      // prevent sending multiple requests in a short time
      if(this.delayLoading && this.readyQueueForLoadPosts === false) {
        console.log('suspend loading more posts 2');
        this.readyQueueForLoadPosts = true;
        return;
      }

      console.log('load more posts');
      this.loadPosts(this.filters.page+1);
    },
    getVideoPreviewUrl(videoUrl) {
      return videoUrl+'?t=0.01';
    },
    getShowGameUrl(serial) {
      return this.showGameEndpoint.replace('_serial', serial);
    },
    getShowRankUrl(serial) {
      return this.showRankEndpoint.replace('_serial', serial);
    },
  }
}
</script>