<template>

  <!--  Main -->
  <ValidationObserver v-slot="{ invalid }" v-if="post">
    <h2 class="mt-3 mb-3">比賽基本資訊
      <button class="btn btn-primary float-right" v-if="!isEditing" @click="clickEdit">
        <i class="fas fa-edit"></i>編輯
      </button>
      <button class="btn btn-primary float-right" v-else
              @click="savePost" :disabled="invalid || loading[SAVING_POST]">
        <i class="fas fa-save" v-if="!loading[SAVING_POST]"></i>
        <i class="fas fa-spinner fa-spin" v-if="loading[SAVING_POST]"></i>
        儲存
      </button>
    </h2>
    <form @submit.prevent>
      <div class="form-group row">
        <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="title">
          標題
        </label>
        <div class="col-sm-10 col-md-11">
          <ValidationProvider rules="required" v-slot="{ errors }">
            <input type="text" class="form-control" id="title" v-model="post.title" required
                   :disabled="!isEditing || loading[SAVING_POST]" maxlength="40"
            >
            <small id="title-help" class="form-text text-muted">
              比賽標題 (40字內)
            </small>
            <span class="text-danger">{{ errors[0] }}</span>
          </ValidationProvider>
        </div>
      </div>
      <div class="form-group row">
        <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="description">
          描述
        </label>
        <div class="col-sm-10 col-md-11">
          <ValidationProvider rules="required" v-slot="{ errors }">
                      <textarea class="form-control" id="description" v-model="post.description" rows="3"
                                style="resize: none"
                                maxlength="100"
                                aria-describedby="description-help"
                                required
                                :disabled="!isEditing || loading[SAVING_POST]">

                      </textarea>
            <small id="description-help" class="form-text text-muted">
              簡單描述這個比賽內容 (100字內)
            </small>
            <span class="text-danger">{{ errors[0] }}</span>
          </ValidationProvider>
        </div>
      </div>
      <div class="form-group row">
        <label class="col-sm-2 col-md-1 col-form-label-lg pt-0">
          發佈
        </label>
        <div class="col-sm-10 col-md-11" v-if="isEditing">
          <ValidationProvider rules="required" v-slot="{ errors }">
            <label class="btn btn-outline-dark" for="post-privacy-public">
              <input type="radio" id="post-privacy-public" v-model="post.policy" value="public"
                     :disabled="loading[SAVING_POST]"
              >

              公開
            </label>

            <label class="btn btn-outline-dark" for="post-privacy-private">
              <input type="radio" id="post-privacy-private" v-model="post.policy" value="private"
                     :disabled="loading[SAVING_POST]"
              >
              私人
            </label>
            <span class="text-danger">{{ errors[0] }}</span>
          </ValidationProvider>
        </div>
        <div class="col-sm-10 col-md-11" v-else>
          <label v-if="post.policy === 'public'">公開</label>
          <label v-if="post.policy === 'private'">私人</label>
        </div>
      </div>
    </form>
  </ValidationObserver>
</template>

<script>
  import moment from 'moment';

  const LOADING_POST = 0;
  const SAVING_POST = 1;

  // import { localize } from 'vee-validate';
  export default {
    created() {
      this.LOADING_POST = LOADING_POST;
      this.SAVING_POST = SAVING_POST;
    },
    mounted() {
      this.loadPost();

      // localize('zh_TW');
      //debug
      // this.serial = 'a14b32';
    },
    props: {
      showPostEndpoint: String,
      updatePostEndpoint: String
    },
    data: function () {
      return {
        loading: {
          LOADING_POSTS: true,
          SAVING_POST: false,
          UPLOADING_VIDEO: false
        },
        post: null,
        isEditing: false,
      }
    },
    methods: {
      loadPost: function () {
        this.uploadLoadingStatus(LOADING_POST, true);
        const url = this.showPostEndpoint;
        axios.get(url)
          .then(res => {
            this.post = res.data.data;
          })
          .finally(() => {
            this.uploadLoadingStatus(LOADING_POST, false);
          });

      },
      savePost: function () {
        this.uploadLoadingStatus(SAVING_POST, true);
        const data = {
          title: this.post.title,
          description: this.post.description,
          policy: {
            access_policy: this.post.policy
          }
        };

        axios.put(this.updatePostEndpoint, data)
          .then(res => {
            this.post = res.data.data;
            this.isEditing = false;
          })
          .finally(() => {
            this.uploadLoadingStatus(SAVING_POST, false);
          });
      },

    }
  }

</script>
