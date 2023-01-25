<template>

  <!--  Main-->
  <div class="container">
    <div>
      <h2 class="d-inline">{{ $t('my_games.title') }}</h2>
      <button class="btn btn-primary d-inline float-right" @click="openCreateFormModal">{{ $t('my_games.new')}}</button>
    </div>

    <!-- Loading-->
    <div class="fa-3x text-center" v-if="loading[LOADING_POSTS]">
      <i class="fas fa-spinner fa-spin"></i>
    </div>

    <div class="mt-2" v-if="!loading[LOADING_POSTS]">
      <table class="table table-hover" v-if="posts.data.length > 0">
        <thead>
        <tr>
          <th scope="col">{{ $t('my_games.table.title')}}</th>
          <th scope="col">{{ $t('my_games.table.description')}}</th>
          <th scope="col">{{ $t('my_games.table.publish') }}</th>
          <th scope="col"></th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="post in posts.data">
          <td>{{post.title}}</td>
          <td>{{post.description}}</td>
          <td>{{post._.policy}}</td>
          <td>
            <a :href="getEditLink(post.serial)"><i class="fas fa-edit"></i></a>
          </td>
        </tr>
        </tbody>
      </table>
    </div>


    <div class="mt-2" v-if="!loading[LOADING_POSTS]">
      <div class="alert alert-info" v-if="posts.data.length === 0">
        {{ $t('my_games.table.no_data')}}
      </div>
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
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <input type="text" class="form-control" id="title" v-model="createPostForm.title" required
                               maxlength="40">
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>

                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="description">
                        {{ $t('Description') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                      <textarea class="form-control" id="description" v-model="createPostForm.description" rows="3"
                                style="resize: none"
                                maxlength="100"
                                aria-describedby="description-help"
                                required></textarea>
                        <small id="description-help" class="form-text text-muted">
                          {{ $t('create_game.description.hint')}}
                        </small>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>

                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('create_game.publish') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <label class="btn btn-outline-dark" for="post-privacy-public">
                          <input type="radio" id="post-privacy-public" v-model="createPostForm.policy.access_policy"
                                 value="public" checked>
                          {{$t('Public')}}
                        </label>
                        <label class="btn btn-outline-dark" for="post-privacy-private">
                          <input type="radio" id="post-privacy-private" v-model="createPostForm.policy.access_policy"
                                 value="private">
                          {{$t('Private')}}
                        </label>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
              </form>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" data-dismiss="modal">{{$t('create_game.cancel')}}</button>
              <button type="submit" class="btn btn-primary float-right" @click="createPost"
                      :disabled="invalid || loading[CREATING_POST]">
                {{$t('create_game.submit')}}
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
      this.loadPost();

      // localize('zh_TW');
      //debug
      // this.serial = 'a14b32';
    },
    props: {
      getPostsEndpoint: String,
      editPostRoute: String,
      createPostEndpoint: String,
    },
    data: function () {
      return {
        createPostForm: {
          title: null,
          description: null,
          policy: {
            access_policy: null
          }
        },
        loading: {
          [LOADING_POSTS]: true,
          [CREATING_POST]: false
        },
        posts: null,
      }
    },
    methods: {
      uploadLoadingStatus(key, status) {
        this.$set(this.loading, key, status);
      },
      loadPost: function () {
        // if(this.$cookies.isKey('creating_post_serial')){
        //   this.serial = this.$cookies.get('creating_post_serial');
        //   const url = this.showPostEndpoint.replace('_serial', this.serial);
        //   axios.get(url)
        //     .then(res => {
        //       const data = res.data.data;
        //       this.title = data.title;
        //       this.serial = data.serial;
        //       this.description = data.description;
        //       this.policy = data.policy;
        //     });
        // }
        this.uploadLoadingStatus(LOADING_POSTS, true);
        const url = this.getPostsEndpoint;
        axios.get(url)
          .then(res => {
            this.posts = res.data;
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
      }
    }
  }

</script>
