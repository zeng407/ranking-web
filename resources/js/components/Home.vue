<script>
import masonry from 'masonry-layout';
import moment from 'moment';
import Vue from 'vue';

const mobileScreenWidth = 991;
export default {
  name: 'home',
  mounted() {
    this.registerHandleRedirectPostSubmit();
    this.registerScrollEvent();
    this.registerInitSearch();
    this.initSorter();
    this.initGoogleAds();
    // this.initMasonry();
    this.loadTags();
    this.getChampions();
    this.handleNewChampion();
    this.autoRefreshChampionsTimestamp();
    window.addEventListener('resize', this.updateMobileScreen);
    history.scrollRestoration = 'manual'; // Disable automatic scroll restoration

  },
  props: {
    indexPostsEndpoint: {
      type: String,
      required: true
    },
    indexTagsEndpoint: {
      type: String,
      required: true
    },
    getChampionsEndpoint: {
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
      tags: [],
      timeRangeText: '',
      errorImages: [],
      isSearching: false,
      isLoadingMorePosts: false,
      delayLoading: false,
      readyQueueForLoadPosts: false,
      posts: [],
      last_page: null,
      lastScrollPosition: 0,
      showReturnUpButton: false,
      champions: [],
      refreshKey: 0,
      championLoading:[],
      mobileScreen: window.innerWidth <= mobileScreenWidth,
      postRefreshKey: 0
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
    loadPosts(page, concat = false) {
      // prevent multiple requests
      if(this.isLoadingMorePosts || this.isFetchAllPosts) {
        return;
      }
      this.isLoadingMorePosts = true;
      this.delayLoading = true;
      const params = this.normalizeParams(page);
      this.sendIndexPostsRequest(params, true)
        .catch(error => {
          this.resetLoading();
          console.error(error);
        });
    },
    sendIndexPostsRequest(params, concat) {
      return axios.get(this.indexPostsEndpoint, {
        params: params
      }).then(response => {
        this.resetLoading();
        this.filters.page = response.data.current_page;
        this.last_page = response.data.last_page;
        if(concat){
          this.posts = this.posts.concat(response.data.data);
        }else{
          this.posts = response.data.data;
        }
        //update posts
        Vue.set(this, 'posts', this.posts);
        Vue.nextTick(() => {
          // this.initMasonry();
        })
      })
    },
    normalizeParams(page) {
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
      return params;
    },
    loadTags() {
      axios.get(this.indexTagsEndpoint)
        .then(response => {
          this.tags = _.orderBy(Object.entries(response.data), [1], ['desc']).map(([name, count]) => ({
            name,
            count
          })); // 'desc' for descending order
        })
        .catch(error => {
          console.error(error);
        });
    },
    addRefreshKey(){
      return this.postRefreshKey++;
    },
    resetLoading() {
      setTimeout(() => {
        this.isSearching = false;
      },300);
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
      // update current sort text after swtich sortBy
      this.initSorter();

      this.isSearching = true;

      const params = this.normalizeParams(1);
      this.sendIndexPostsRequest(params, false)
      .then(() => {
        // remove preload-posts by class
        document.querySelectorAll('.preload-post').forEach((element) => {
          element.remove();
        });
      });

      this.scrollToSorter();
    },
    scrollToSorter() {
      const sorter = document.getElementById('sorter-hr');
      if(sorter){
        window.scrollTo({
          top: sorter.offsetTop,
          behavior: 'smooth'
        });
      }
    },
    addTag(tag) {
      this.filters.keyword = tag;
      // update keyword-input value
      document.getElementById('keyword-input').value = tag;
      this.search();
    },
    clearKeyword() {
      this.filters.keyword = '';
      // update keyword-input value
      document.getElementById('keyword-input').value = '';
      this.search();
    },
    share(url, id) {
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
      let buffer = 200;
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
      //handle event when google-ads is loaded
      window.addEventListener('ad-loaded', () => {
        if (window.adsbygoogle) {
          // this.initMasonry();
        }
      });
    },
    initSorter() {
      if (this.filters.sort_by === 'hot') {
        if (this.filters.range === 'all') {
          this.timeRangeText = this.$t('All Time');
        } else if (this.filters.range === 'week') {
          this.timeRangeText = this.$t('This Week');
        } else if (this.filters.range === 'day') {
          this.timeRangeText = this.$t('Today');
        } else if (this.filters.range === 'month') {
          this.timeRangeText = this.$t('This Month');
        } else if (this.filters.range === 'year') {
          this.timeRangeText = this.$t('This Year');
        }
      }
    },
    registerInitSearch() {
      this.$bus.$on('initiate-search', ($event) => {
        const parentForm = $event.target.closest('form');
        const formData = new FormData(parentForm);
        const keyword = formData.get('k');
        this.filters.keyword = keyword;
        this.search();
      });
    },
    registerHandleRedirectPostSubmit() {
      window.addEventListener('beforeunload', function (e) {
        // 僅在 reload 時觸發
        if (performance.getEntriesByType('navigation')[0]?.type === 'reload') {
          // 用 GET 方式重新導向
          window.location.assign(window.location.href);
        }
      })
    },
    registerScrollEvent() {
      window.addEventListener('scroll', this.handleScroll);
    },
    triggerLoadPosts() {
      if(this.isLoadingMorePosts || this.readyQueueForLoadPosts || this.isFetchAllPosts) {
        return;
      }

      // prevent sending multiple requests in a short time
      if(this.delayLoading && this.readyQueueForLoadPosts === false) {
        this.readyQueueForLoadPosts = true;
        return;
      }

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
    getChampions() {
      axios.get(this.getChampionsEndpoint)
        .then(response => {
          let champions = response.data;
          //order by created_at
          champions.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
          this.champions = champions;
        })
        .catch(error => {
          console.error(error);
        });
    },
    handleNewChampion() {
      Echo.channel('home.champion')
        .listen('.new-champion', (data) => {
            // filter out the champion that already exists by id
            if(this.champions.find(champion => champion.key === data.key)){
              return;
            }

            // if data.left is in the champions's left or right , skip push to championLoading
            if(this.champions.find(champion => champion.left.thumb_url === data.left.thumb_url || champion.right.thumb_url === data.left.thumb_url)){
              // skip push to championLoading
            }else{
              this.championLoading.push(data.left);
            }

            // if data.right is in the champions's left or right , skip push to championLoading
            if(this.champions.find(champion => champion.left.thumb_url === data.right.thumb_url || champion.right.thumb_url === data.right.thumb_url)){
              // skip push to championLoading
            }else{
              this.championLoading.push(data.right);
            }

            // push data to the front of the array
            this.champions.unshift(data);

            // max size of champions is 15
            if(this.champions.length > 15){
              // remove elements after 15
              this.champions = this.champions.slice(0, 15);
            }
        });
    },
    humanizeDate(date) {
      moment.locale(this.$i18n.locale);
      // date never greater than now
      if(moment(date).isAfter(moment())){
        return moment().fromNow();
      }
      return moment(date).fromNow();
    },
    isEndWith(str, suffix) {
      return str.endsWith(suffix);
    },
    autoRefreshChampionsTimestamp() {
      setInterval(() => {
        //reredner the champions timestamp
        this.refreshKey++
      }, 60 * 1000);
    },
    handleCandicateLoaded(candicate) {
      this.championLoading = this.championLoading.filter(champion => champion !== candicate);
    },
    isChampionLoading(candicate) {
      return this.championLoading.includes(candicate);
    },
    updateMobileScreen() {
        this.mobileScreen = window.innerWidth <= mobileScreenWidth;
    },
  }
}
</script>
