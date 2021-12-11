<template>
  <div class="container">

    <div class="d-inline-block">
      <h2 class="float-left">管理我的比賽</h2>
      <button class="btn btn-primary float-right">新增比賽</button>
    </div>
    <table class="table table-hover">
      <thead>
      <tr>
        <th scope="col">標題</th>
        <th scope="col">描述</th>
        <th scope="col">公開</th>
        <th scope="col"></th>
      </tr>
      </thead>
      <tbody v-if="posts && !loading[LOADING_POSTS]">
      <tr v-for="post in posts.data">
        <td>{{post.title}}</td>
        <td>{{post.description}}</td>
        <td>{{post.policy}}</td>
        <td>
          <a href="#" @click="clickEditPost(post)"><i class="fas fa-edit">編輯</i></a>
        </td>
      </tr>
      </tbody>
    </table>

    <div class="modal fade" id="exampleModal" tabindex="-1" aria-labelledby="exampleModalLabel" aria-hidden="true">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="exampleModalLabel">比賽基本資訊</h5>
            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <ValidationObserver v-slot="{ invalid }" v-if="clickedPost">
          <div class="modal-body">
              <form @submit.prevent>
                <div class="form-group row">
                  <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="title">
                    標題
                  </label>
                  <div class="col-sm-10 col-md-11">
                    <ValidationProvider rules="required" v-slot="{ errors }">
                      <input type="text" class="form-control" id="title" v-model="clickedPost.title" required>
                      <span class="text-danger">{{ errors[0] }}</span>
                    </ValidationProvider>
                  </div>
                </div>
                <div class="form-group row">
                  <label class="col-sm-2 col-md-1 col-form-label-lg pt-0" for="description">
                    描述
                  </label>
                  <div class="col-sm-10 col-md-11">
                  <textarea class="form-control" id="description" v-model="clickedPost.description" rows="3"
                            style="resize: none"
                            maxlength="200"
                            aria-describedby="description-help"
                            required></textarea>
                    <small id="description-help" class="form-text text-muted">
                      簡單描述這個比賽內容
                    </small>
                  </div>
                </div>
                <div class="form-group row">
                  <label class="col-sm-2 col-md-1 col-form-label-lg pt-0">
                    公開
                  </label>
                  <div class="col-sm-10 col-md-11">
                    <div>
                      <label class="btn btn-outline-dark" for="post-privacy-public">
                        <input type="radio" id="post-privacy-public" v-model="clickedPost.policy" value="public" checked>
                        所有人都可以看到
                      </label>
                    </div>
                    <div>
                      <label class="btn btn-outline-dark" for="post-privacy-private">
                        <input type="radio" id="post-privacy-private" v-model="clickedPost.policy" value="private">
                        只有自己可以看到
                      </label>
                    </div>
                  </div>
                </div>
              </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            <input type="submit" class="btn btn-primary float-right" @click="savePost" :disabled="invalid || loading[SAVING_POST]"
                   value="儲存">
          </div>
          </ValidationObserver>
        </div>
      </div>
    </div>

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

  const LOADING_POSTS = 0;
  const SAVING_POST = 1;

  // import { localize } from 'vee-validate';
  export default {
    mounted() {
      bsCustomFileInput.init();
      this.loadPost();

      // localize('zh_TW');
      //debug
      // this.serial = 'a14b32';
    },
    props: {
      getPostsEndpoint: String,
      showPostEndpoint: String,
      createPostEndpoint: String,
      updatePostEndpoint: String,
      updateElementEndpoint: String,
      createImageElementEndpoint: String,
      createVideoElementEndpoint: String
    },
    data: function () {
      return {
        title: null,
        description: null,
        policy: 'public',
        serial: null,
        loading: {
          loading_posts: true,
          saving_post: false
        },
        uploadingFiles: {},
        elements: [],
        posts: null,
        clickedPost: null
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
        this.uploadLoadingStatus('loading_posts', true);
        const url = this.getPostsEndpoint;
        axios.get(url)
          .then(res => {
            this.posts = res.data;
          })
          .finally(() => {
            this.uploadLoadingStatus('loading_posts', false);
          });

      },
      clickPostRow: function (post) {
        this.clickedPost = _.clone(post);

        console.log(post);
      },
      clickEditPost: function (post) {
        this.clickedPost = _.clone(post);
        $('#exampleModal').modal('show');
        console.log(post);
      },
      savePost: function () {
        this.uploadLoadingStatus(SAVING_POST, true);
        const data = {
          title: this.clickedPost.title,
          description: this.clickedPost.description,
          policy: this.clickedPost.policy
        };

        if (this.serial === null) {
          axios.post(this.createPostEndpoint, data)
            .then(res => {
              this.serial = res.data.serial;
              this.$cookies.set('creating_post_serial')
            })
            .finally(() => {
              this.uploadLoadingStatus(SAVING_POST, false);
            });
        } else {
          const url = this.updatePostEndpoint.replace('_serial', this.serial);
          axios.put(url, data)
            .then(res => {
              this.serial = res.data.serial;
            })
            .finally(() => {
              this.uploadLoadingStatus(SAVING_POST, false);
            });
        }
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
