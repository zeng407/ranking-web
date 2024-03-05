<template>
  <div class="mt-5 container-fluid">
    <div class="form-inline w-100">
      <form @submit.prevent class="d-flex w-100 justify-content-center">
        <div class="mr-2 w-50">
          <input class="form-control w-100" v-model="filters.any_like" type="search" placeholder="Search"
            aria-label="Search">
          <div>
            <div class="d-flex mt-1 w-100">
              <button type="button" class="btn btn-sm btn-outline-secondary mr-1" v-for="tag in hotTags" @click="addTag(tag)">
                #{{ tag.name }} 
                <!-- <span class="badge badge-light">0</span> -->
              </button>
            </div>
          </div>
        </div>
        <div class="">
          <button class="btn btn-outline-dark w-100" type="submit" @click="search">
            <i class="fas fa-search"></i>
          </button>
        </div>
      </form>
    </div>

    <div class="d-flex justify-content-between flex-nowrap mt-4">
      <div class="form-inline">
        <div class="btn-group btn-group-toggle" data-toggle="buttons">

          <label class="btn btn-outline-dark mr-2">
            <input type="radio" v-model="sortBy" value="hot">
            <i class="fas fa-fire-alt"></i>&nbsp;{{ $t('Hot') }}
          </label>

          <label class="btn btn-outline-dark mr-2">
            <input type="radio" v-model="sortBy" value="new">
            <i class="fas fa-sort-amount-down-alt"></i>&nbsp;{{ $t('New') }}
          </label>
        </div>
      </div>

    </div>
    <div class="d-flex justify-content-start">
      <div class="form-inline mt-3">
        <button type="button" class="btn btn-outline-dark btn-sm rounded-pill" data-toggle="dropdown"
          v-show="sortBy === 'hot'" style="top: 0">
          {{ timeRangeText }}
          <i class="fas fa-caret-down"></i>
        </button>
        <div class="dropdown-menu">
          <a class="dropdown-item" @click="clickTimeRange($event, 'all')" href="#">{{ $t('All') }}</a>
          <a class="dropdown-item" @click="clickTimeRange($event, 'year')" href="#">{{ $t('This Year') }}</a>
          <a class="dropdown-item" @click="clickTimeRange($event, 'month')" href="#">{{ $t('This Month') }}</a>
          <a class="dropdown-item" @click="clickTimeRange($event, 'week')" href="#">{{ $t('This Week') }}</a>
          <a class="dropdown-item" @click="clickTimeRange($event, 'day')" href="#">{{ $t('Today') }}</a>
        </div>
      </div>
    </div>

    <div class="row justify-content-center pt-4">
      <div class="fa-3x" v-if="isLoading">
        <i class="fas fa-spinner fa-spin"></i>
      </div>

      <div class="col-xl-4 col-md-6 pt-2" v-if="!isLoading" v-for="post in posts.data">
        <div class="card">
          <div class="card-header text-center">
            <h3>{{ post.title }}</h3>
          </div>
          <div class="row no-gutters">
            <div class="col-6">
              <div :style="{
                'background': 'url(' + post.image1.url + ')',
                'width': '100%',
                'height': '300px',
                'background-repeat': 'no-repeat',
                'background-size': 'cover',
                'background-position': 'center center',
                'display': 'flex'
              }"></div>
              <p class="text-center mt-1">{{ post.image1.title }}</p>
            </div>
            <div class="col-6">
              <div :style="{
                'background': 'url(' + post.image2.url + ')',
                'width': '100%',
                'height': '300px',
                'background-repeat': 'no-repeat',
                'background-size': 'cover',
                'background-position': 'center center',
                'display': 'flex'
              }"></div>

              <p class="text-center mt-1">{{ post.image2.title }}</p>
            </div>
            <div class="card-body pt-0 text-center">
              <h5 class="text-break">{{ post.description }}</h5>
              <div class="row">
                <div class="col-6">
                  <a class="btn btn-primary btn-block" :href="getGameUrl(post.serial)" target="_blank">
                    <i class="fas fa-play"></i> Play
                  </a>
                </div>
                <div class="col-6">
                  <a class="btn btn-secondary btn-block" :href="getRankUrl(post.serial)" target="_blank">
                    <i class="fas fa-trophy"></i> Rank
                  </a>
                </div>
              </div>
              <span class="mt-2 card-text float-left">
                <button type="button" class="btn btn-outline-dark btn-sm" v-b-popover.right.click="'Copied'"
                  @click="copyGameUrl(post.serial, $event)">
                  Link &nbsp;<i class="fas fa-share-square"></i>
                </button>
              </span>
              <span class="mt-2 card-text float-right">
                <span class="pr-2">
                  <i class="fas fa-play-circle"></i>&nbsp;{{ post.play_count }}
                </span>
                <small class="text-muted">{{ post.created_at | datetime }}</small>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row justify-content-center pt-2">
      <div v-if="posts.data.length && posts.meta.last_page > 1">
        <b-pagination v-model="currentPage" :total-rows="posts.meta.total" :per-page="posts.meta.per_page" first-number
          last-number @change="handlePageChange" align="center"></b-pagination>
      </div>
    </div>

  </div>
</template>

<script>
import { debounceMixin } from './helper/debounce.js';

export default {
  mounted() {
    this.loadData(1);
    this.loadTagsOptions();
    window.addEventListener('scroll', this.handleScroll);
  },
  props: {
    indexPostsEndpoint: String,
    playGameRoute: String,
    gameRankRoute: String,
    sort: {
      type: String,
      default: 'hot'
    },
    getTagsOptionsEndpoint: String
  },
  data: function () {
    return {
      posts: {
        data: [],
        links: [],
        meta: {
          from: 1,
          to: 1
        }
      },
      filters: {
        any_like: ''
      },
      sortBy: this.sort,
      timeRange: 'month',
      timeRangeText: this.$t('This Month'),
      isLoading: false,
      currentPage: 1,
      lastPage: 1,
      hotTags: [],
    }
  },
  watch: {
    sortBy: function (value) {
      if (value === 'hot') {
        window.history.pushState('', '', '/hot');
      } else if (value === 'new') {
        window.history.pushState('', '', '/new');
      }
      this.loadData(1);
    }
  },
  mixins: [debounceMixin],
  methods: {
    loadData: function (page) {
      this.isLoading = true;

      let sortParam = null;
      if (this.sortBy === 'hot') {
        sortParam = 'hot_' + this.timeRange;
      } else if (this.sortBy === 'new') {
        sortParam = 'new';
      }

      const param = _.assign(this.filters, {
        page: page,
        sort_by: sortParam
      });

      axios.get(this.indexPostsEndpoint, {
        params: param
      })
        .then(res => {
          this.posts = res.data;
          this.currentPage = res.data.meta.current_page;
          this.lastPage = res.data.meta.last_page;
          // Vue.nextTick(() => {
          //   $('[data-toggle="popover"]').popover().click(() => {
          //     setTimeout(() => {
          //       $('[data-toggle="popover"]').blur();
          //       $('[data-toggle="popover"]').popover('hide');
          //     }, 1000);
          //   });
          // });
        })
        .finally(() => {
          this.isLoading = false;
        });
    },
    loadTagsOptions: function () {
      axios.get(this.getTagsOptionsEndpoint)
        .then(res => {
          this.hotTags = res.data;
        });
    },
    addTag: function (tag) {
      this.filters.any_like = tag.name;
      this.loadData(1);
    },
    showPost: function (serial) {
      const url = this.playGameRoute.replace('_serial', serial);
      window.open(url, '_blank');
    },
    getGameUrl: function (serial) {
      return this.playGameRoute.replace('_serial', serial);
    },
    getRankUrl: function (serial) {
      return this.gameRankRoute.replace('_serial', serial);
    },
    copyGameUrl: function (serial, event) {
      navigator.clipboard.writeText(this.getGameUrl(serial));
      setTimeout(() => {
        this.$root.$emit('bv::hide::popover');
      }, 1000);

    },
    search: function () {
      this.isLoading = true;
      this.debounce(() => {
        this.loadData(1);
      }, 300)();
    },
    handlePageChange: function (page) {
      this.loadData(page);
    },
    clickTimeRange: function (event, value) {
      this.timeRange = value;
      this.timeRangeText = event.target.text;
      Vue.nextTick(() => {
        this.loadData(1);
      });
    }
  }
}

</script>
