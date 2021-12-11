<template>
  <!--  Loading-->
  <div class="fa-3x text-center" v-if="loading[LOADING_POST]">
    <i class="fas fa-spinner fa-spin"></i>
  </div>

  <!--  Main-->
  <div class="container" v-else>
    <ValidationObserver v-slot="{ invalid }" v-if="post">
      <h2 class="mt-3 mb-3">比賽基本資訊
        <button class="btn btn-primary float-right" v-if="!isEditing" @click="clickEdit">
          <i class="fas fa-edit"></i>編輯
        </button>
        <button class="btn btn-primary float-right" v-else
                @click="savePost" :disabled="invalid || loading[SAVING_POST]">
          <i class="fas fa-save"></i>儲存
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
                     :disabled="!isEditing"
              >
              <span class="text-danger">{{ errors[0] }}</span>
            </ValidationProvider>
          </div>
        </div>
        <div class="form-group row">
          <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="description">
            描述
          </label>
          <div class="col-sm-10 col-md-11">
            <textarea class="form-control" id="description" v-model="post.description" rows="3"
                      style="resize: none"
                      maxlength="200"
                      aria-describedby="description-help"
                      required
                      :disabled="!isEditing">

            </textarea>
            <small id="description-help" class="form-text text-muted">
              簡單描述這個比賽內容
            </small>
          </div>
        </div>
        <div class="form-group row">
          <label class="col-sm-2 col-md-1 col-form-label-lg pt-0">
            權限
          </label>
          <div class="col-sm-10 col-md-11" v-if="isEditing">

            <label class="btn btn-outline-dark" for="post-privacy-public">
              <input type="radio" id="post-privacy-public" v-model="post.policy" value="public"
              >
              所有人都可以看到
            </label>

            <label class="btn btn-outline-dark" for="post-privacy-private">
              <input type="radio" id="post-privacy-private" v-model="post.policy" value="private"
              >
              只有自己可以看到
            </label>
          </div>
          <div class="col-sm-10 col-md-11" v-else>
            <label v-if="post.policy === 'public'">所有人都可以看到</label>
            <label v-if="post.policy === 'private'">只有自己可以看到</label>
          </div>
        </div>
      </form>
    </ValidationObserver>


    <h2 class="mt-3 mb-3">上傳素材</h2>
    <div class="row">
      <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="description">
        圖片
      </label>
      <div class="col-sm-10 col-md-11">
        <div class="custom-file">
          <input type="file" class="custom-file-input" id="image-upload" multiple
                 @change="uploadImages">
          <label class="custom-file-label" for="image-upload">Choose file</label>
        </div>
      </div>
    </div>
    <div class="progress mb-1" v-for="(progress, name) in uploadingFiles">
      <div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" aria-valuemin="0"
           aria-valuemax="100"
           :aria-valuenow="progress" :style="{'width': progress+'%'}">
        {{name}}
      </div>
    </div>

    <h2 class="mt-3 mb-3">編輯素材</h2>
    <div class="row">
      <div class="col-sm-6 col-lg-4" v-for="element in elements">
        <div class="card mb-3">
          <img :src="element.thumb_url" class="card-img-top" alt="">
          <div class="card-body">
            <!--            <h5 class="card-title">{{element.title}}</h5>-->
            <!--            <textarea class="form-control-plaintext overflow-hidden" type="text" rows="3" maxlength="40" style="resize:none"-->
            <!--                      @change="updateElement(element.id, $event)">{{element.title}}</textarea>-->
            <input class="form-control-plaintext" type="text" :value="element.title" maxlength="40">
            <p class="card-text"><small class="text-muted">{{moment(element.created_at).fromNow()}}</small></p>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
  import bsCustomFileInput from 'bs-custom-file-input'

  const LOADING_POST = 0;
  const SAVING_POST = 1;

  // import { localize } from 'vee-validate';
  export default {
    created() {
      this.LOADING_POST = LOADING_POST;
      this.SAVING_POST = SAVING_POST;
    },
    mounted() {
      bsCustomFileInput.init();
      this.loadPost();

      // localize('zh_TW');
      //debug
      // this.serial = 'a14b32';
    },
    props: {
      showPostEndpoint: String,
      updatePostEndpoint: String,
      updateElementEndpoint: String,
      createImageElementEndpoint: String,
      createVideoElementEndpoint: String
    },
    data: function () {
      return {
        loading: {
          LOADING_POSTS: true,
          SAVING_POST: false
        },
        uploadingFiles: {},
        elements: [],
        post: null,
        isEditing: false
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
      clickEdit: function () {
        this.isEditing = true;
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
            this.post = res.data;
            this.isEditing = false;
          })
          .finally(() => {
            this.uploadLoadingStatus(SAVING_POST, false);
          });
      },
      uploadImages: function (event) {
        Array.from(event.target.files).forEach(file => {
          let form = new FormData();
          form.append('file', file);
          form.append('post_serial', this.serial);
          axios.post(this.createImageElementEndpoint, form, {
            onUploadProgress: progressEvent => {
              console.log(progressEvent.loaded / progressEvent.total * 100);
              this.updateProgressBarValue(file, progressEvent);
            }
          })
            .then(res => {
              console.log(res.data);
              this.elements.push(res.data.data);
              this.deleteProgressBarValue(file);
            });
        });
      },
      updateElement: function (id, event) {
        console.log(id);
        console.log(event.target.value);

      },
      updateProgressBarValue: function (file, progressEvent) {
        let filename = file.name;
        this.$set(this.uploadingFiles, filename, Math.round(progressEvent.loaded / progressEvent.total * 100));
      },
      deleteProgressBarValue: function (file) {
        delete this.uploadingFiles[file.name];
        this.uploadingFiles = Object.assign({}, this.uploadingFiles);
      }

    }
  }

</script>
