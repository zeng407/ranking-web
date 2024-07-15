<template>
  <!--  Main-->
  <div class="container">
    <div class="mt-3">
      <h2 class="d-inline">{{ $t('my_games.title') }}</h2>
      <button class="btn btn-primary d-inline float-right" @click="openCreateFormModal">{{ $t('my_games.new') }}
      </button>
    </div>

    <!-- Loading-->
    <div class="fa-3x text-center" v-if="loading[LOADING_POSTS]">
      <i class="fas fa-spinner fa-spin"></i>
    </div>

    <div class="mt-2" v-if="!loading[LOADING_POSTS]">
      <table class="table table-bordered table-hover table-responsive-sm-vertical" v-if="posts.data.length > 0">
        <thead class="th-nowrap">
          <tr>
            <th scope="col" style="width: 40%">{{ $t('my_games.table.title') }}</th>
            <th scope="col" style="width: 30%">{{ $t('my_games.table.description') }}</th>
            <th scope="col">{{ $t('my_games.table.publishment') }}</th>
            <th scope="col">{{ $t('my_games.table.play_count') }}</th>
            <th scope="col">{{ $t('my_games.table.edit') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="post in posts.data" :key="post.serial">
            <td class="text-break" :data-label="$t('my_games.table.title')">{{ post.title }}</td>
            <td class="text-break" :data-label="$t('my_games.table.description')">{{ post.description  }}</td>
            <td class="text-break" :data-label="$t('my_games.table.publishment')">
              {{ $t('post_policy.'+post.policy) }}
              <i v-if="post.policy == 'password'" class="fas fa-lock"></i>
              <i v-else-if="post.policy == 'private'" class="fas fa-user-lock"></i>
              <i v-else-if="post.policy == 'public'" class="fas fa-globe"></i>
            </td>
            <td class="text-break" :data-label="$t('my_games.table.play_count')">
              <span class="pr-2 badge badge-secondary">
                {{ $t('my_games.table.played_all') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.play_count }}
              </span>
              <br>
              <!-- <span class="pr-2 badge badge-secondary">
                {{ $t('my_games.table.played_last_week') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.last_week_play_count }}
              </span>
              <br> -->
              <span class="pr-2 badge badge-secondary">
                {{ $t('my_games.table.played_this_week') }}&nbsp;<i class="fas fa-play-circle"></i>&nbsp;{{ post.this_week_play_count }}
              </span>
            </td>
            <td :data-label="$t('my_games.table.edit')">
              <a :title="$t('my_games.table.edit')" :href="getEditLink(post.serial)"><i class="fa-xl fas fa-edit"></i></a>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="row">
      <div class="col-12" v-if="!loading[LOADING_POSTS] && posts.meta.total > 0 && posts.meta.last_page > 1">
        <b-pagination v-model="currentPage" :total-rows="posts.meta.total" :per-page="posts.meta.per_page" first-number
          last-number @change="handlePageChange" align="center"></b-pagination>
      </div>
    </div>



    <div class="mt-2" v-if="!loading[LOADING_POSTS] && posts.data.length == 0">
      <table class="table table-bordered table-hover" style="table-layout: fixed">
        <thead class="th-nowrap">
          <tr>
            <th scope="col" style="width: 40%">{{ $t('my_games.table.title') }}</th>
            <th scope="col" style="width: 30%">{{ $t('my_games.table.description') }}</th>
            <th scope="col">{{ $t('my_games.table.publishment') }}</th>
            <th scope="col">{{ $t('my_games.table.edit') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td colspan="4" class="text-center">{{ $t('my_games.table.no_data') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- modal -->
    <div class="modal fade" id="modal" tabindex="-1" aria-labelledby="modalLabel" aria-hidden="true">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="modalLabel">{{ $t('create_game.title') }}</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <ValidationObserver v-slot="{ invalid }">
            <div class="modal-body">
              <form @submit.prevent>

                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="title">
                        {{ $t('Title') }}
                      </label>
                      <ValidationProvider rules="required" :name="$t('Title')" v-slot="{ errors }">
                        <input type="text" class="form-control" id="title" v-model="createPostForm.title" required
                          :maxlength="maxPostTitleLength">
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                      <CountWords :words="createPostForm.title" :maxLength="maxPostTitleLength"></CountWords>
                    </div>
                  </div>
                </div>

                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="description">
                        {{ $t('Description') }}
                      </label>
                      <ValidationProvider rules="required" :name="$t('Description')" v-slot="{ errors }">
                        <textarea class="form-control" id="description" v-model="createPostForm.description" rows="3"
                          style="resize: none" :maxlength="maxPostDescriptionLength" aria-describedby="description-help" required></textarea>
                        <small id="description-help" class="form-text text-muted">
                          {{ $t('create_game.description.hint') }}
                        </small>
                        <span class="text-danger">{{ errors[0] }}</span>
                        <CountWords :words="createPostForm.description" :maxLength="maxPostDescriptionLength"></CountWords>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>

                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('create_game.publishment') }}
                      </label>
                      <ValidationProvider rules="required" :name="$t('create_game.publishment')" v-slot="{ errors }">
                        <div class="form-group">
                          <label class="btn btn-outline-dark" for="post-privacy-public">
                            <input type="radio" id="post-privacy-public" v-model="createPostForm.policy.access_policy"
                              value="public" checked>
                            {{ $t('post_policy.public') }}
                          </label>
                          <label class="btn btn-outline-dark" for="post-privacy-private">
                            <input type="radio" id="post-privacy-private" v-model="createPostForm.policy.access_policy"
                              value="private">
                            {{ $t('post_policy.private') }}
                          </label>
                          <label class="btn btn-outline-dark" for="post-privacy-password">
                            <input type="radio" id="post-privacy-password" v-model="createPostForm.policy.access_policy"
                              value="password">
                            {{ $t('post_policy.password') }}
                          </label>
                          <span class="text-danger">{{ errors[0] }}</span>
                        </div>
                      </ValidationProvider>
                      <ValidationProvider v-if="createPostForm.policy.access_policy == 'password'" :name="$t('create_game.password')" rules="required" v-slot="{ errors }">
                        <label class="col-form-label-lg" for="post-password">
                          {{ $t('create_game.password') }}
                          <input id="post-password" type="text" class="form-control" v-model="createPostForm.policy.password" maxlength="255">
                        </label>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
              </form>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" data-dismiss="modal">{{ $t('create_game.cancel') }}
              </button>
              <button type="submit" class="btn btn-primary float-right" @click="createPost"
                :disabled="invalid || loading[CREATING_POST]">
                {{ $t('create_game.submit') }}
                <span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"
                  v-if="loading[CREATING_POST]"></span>
              </button>
            </div>
          </ValidationObserver>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import CountWords from './partials/CountWords.vue';


const LOADING_POSTS = 0;
const CREATING_POST = 1;

// import { localize } from 'vee-validate';
export default {
  created() {
    this.LOADING_POSTS = LOADING_POSTS;
    this.CREATING_POST = CREATING_POST;
  },
  mounted() {
    // bsCustomFileInput.init();
    this.loadPosts();

    // localize('zh_TW');
    //debug
    // this.serial = 'a14b32';
  },
  props: {
    getPostsEndpoint: String,
    editPostRoute: String,
    createPostEndpoint: String,
    maxPostTitleLength: String,
    maxPostDescriptionLength: String
  },
  data: function () {
    return {
      createPostForm: {
        title: null,
        description: null,
        policy: {
          access_policy: null,
          password: null
        },
      },
      loading: {
        [LOADING_POSTS]: true,
        [CREATING_POST]: false
      },
      posts: null,
      currentPage: 1
    }
  },
  methods: {
    uploadLoadingStatus(key, status) {
      this.$set(this.loading, key, status);
    },
    loadPosts: function (page) {
      const filter = {
        'page': page
      };
      this.uploadLoadingStatus(LOADING_POSTS, true);
      const url = this.getPostsEndpoint;
      axios.get(url, { params: filter })
        .then(res => {
          this.posts = res.data;
          this.currentPage = res.data.meta.current_page;
        })
        .finally(() => {
          this.uploadLoadingStatus(LOADING_POSTS, false);
        });

    },
    getEditLink: function (serial) {
      return this.editPostRoute.replace('_serial', serial);
    },
    openCreateFormModal: function () {
      $('#modal').modal('show');
    },
    createPost: function () {
      this.uploadLoadingStatus(CREATING_POST, true);
      axios.post(this.createPostEndpoint, this.createPostForm)
        .then(res => {
          window.open(this.getEditLink(res.data.serial), '_self');
          // this.loadPost();
          // $('#modal').modal('hide');
          // this.resetPostForm();
        })
        .finally(() => {
          // this.uploadLoadingStatus(CREATING_POST, false);
        });
    },
    resetPostForm: function () {
      const data = {
        title: null,
        description: null,
        policy: {
          access_policy: null
        }
      };
      this.createPostForm = Object.assign({}, data);
    },
    handlePageChange: function (page) {
      this.loadPosts(page);
    },
  }
}

</script>
