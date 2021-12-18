<template>

  <!--  Main -->
  <div class="container-fluid">

    <!-- Alert -->
    <div class="position-fixed"
         style="top: 20px; right: 15px">
      <span class="cursor-pointer" @click="dismissAlert">
        <b-alert
          :show="dismissCountDown"
          dismissible
          fade
          :variant="alertLevel"
        >
          {{alertText}}
        </b-alert>
      </span>
    </div>

    <!-- tabs -->
    <div class="row">
      <div class="col-3">
        <div class="nav flex-column nav-pills" id="v-pills-tab" role="tablist" aria-orientation="vertical">
          <a class="nav-link active" id="v-pills-home-tab" data-toggle="pill" href="#v-pills-home" role="tab"
             aria-controls="v-pills-home" aria-selected="true">基本資訊</a>
          <a class="nav-link" id="v-pills-profile-tab" data-toggle="pill" href="#v-pills-profile" role="tab"
             aria-controls="v-pills-profile" aria-selected="false">素材</a>
          <a class="nav-link" id="v-pills-messages-tab" data-toggle="pill" href="#v-pills-messages" role="tab"
             aria-controls="v-pills-messages" aria-selected="false">統計</a>
          <a class="nav-link" id="v-pills-settings-tab" data-toggle="pill" href="#v-pills-settings" role="tab"
             aria-controls="v-pills-settings" aria-selected="false">留言</a>
        </div>
      </div>

      <div class="col-9">
        <div class="tab-content" id="v-pills-tabContent">

          <!-- info -->
          <div class="tab-pane fade show active" id="v-pills-home" role="tabpanel" aria-labelledby="v-pills-home-tab">
            <!-- Loading -->
            <div class="col-9 fa-3x text-center" v-if="loading[LOADING_POST]">
              <i class="fas fa-spinner fa-spin"></i>
            </div>

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
          </div>

          <!-- elements -->
          <div class="tab-pane fade" id="v-pills-profile" role="tabpanel" aria-labelledby="v-pills-profile-tab">
            <!-- upload image -->
            <h2 class="mt-3 mb-3">上傳圖片</h2>
            <div class="row">
              <div class="col-12">
                <label>從電腦上傳</label>
                <div class="custom-file form-group">
                  <input type="file" class="custom-file-input" id="image-upload" multiple
                         @change="uploadImages">
                  <label class="custom-file-label" for="image-upload">Choose File...</label>
                </div>
              </div>
            </div>
            <!-- upload progress bar -->
            <div class="progress mb-1" v-for="(progress, name) in uploadingFiles">
              <div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" aria-valuemin="0"
                   aria-valuemax="100"
                   :aria-valuenow="progress" :style="{'width': progress+'%'}">
                {{name}}
              </div>
            </div>

            <!-- upload video -->
            <h2 class="mt-5 mb-3">上傳影片</h2>
            <div class="row">
              <div class="col-12">
                <label for="youtubeURL">Youtube</label>
                <div class="input-group">
                  <input class="form-control" type="text" id="youtubeURL" name="youtubeURL"
                         v-model="uploadVideoUrl"
                         aria-label="https://www.youtube.com/watch?v=dQw4w9WgXcQ" aria-describedby="youtubeUpload"
                         placeholder="https://www.youtube.com/watch?v=dQw4w9WgXcQ">
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" id="youtubeUpload" @click="uploadVideo"
                            :disabled="loading[UPLOADING_VIDEO]">
                      <span v-show="!loading[UPLOADING_VIDEO]">新增</span>
                      <i v-show="loading[UPLOADING_VIDEO]" class="fas fa-spinner fa-spin"></i>

                      <!--                      <i class="fas fa-spinner"></i>-->
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- edit -->
            <h2 class="mt-5 mb-3">編輯素材</h2>
            <div class="row">
              <template v-for="(element, index) in elements">
                <div class="col-lg-4 col-md-6" v-show="element.type==='video' && isElementInPage(index)">
                  <div class="card mb-3">
                    <!-- video -->
                    <youtube v-if="element.type==='video' && element.loadedVideo" width="100%" height="270"
                             :ref="element.id"
                             @ready="doPlay(element)"
                             :player-vars="{
                              controls:1,
                              autoplay:0,
                              start: element.video_start_second,
                              end:element.video_end_second,
                              rel: 0,
                              host: 'https://www.youtube.com'
                              }"
                    ></youtube>
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                         v-if="element.type==='video' && !element.loadedVideo">

                    <div class="card-body">
                      <input class="form-control-plaintext" type="text" :value="element.title" maxlength="100"
                             @change="updateElementTitle(element.id, $event)">
                      <div class="row mb-3">
                        <div class="col-10">
                          <div class="input-group">
                            <div class="input-group-prepend d-lg-none d-xl-block">
                              <span class="input-group-text">播放範圍</span>
                            </div>
                            <input type="text" class="form-control" name="video_start_second"
                                   placeholder="0:00" aria-label="start"
                                   @change="updateVideoScope(index, element, $event)"
                                   :value="toTimeFormat(element.video_start_second)">
                            <div class="input-group-prepend"><span class="input-group-text">~</span></div>
                            <input type="text" class="form-control" name="video_end_second"
                                   :placeholder="toTimeFormat(element.video_duration_second)" aria-label="end"
                                   :value="toTimeFormat(element.video_end_second)"
                                   @change="updateVideoScope(index, element, $event)">
                          </div>
                        </div>
                        <div class="col-2">
                          <a class="btn btn-danger float-right" @click="clickPlayButton(index, element)">
                            <i class="fas fa-play-circle"></i>
                          </a>
                        </div>
                      </div>
                      <span class="card-text"><small
                        class="text-muted">{{moment(element.created_at).format('lll')}}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element, $event)">
                        <i class="fas fa-trash"></i>
                      </a>
                    </div>
                  </div>
                </div>

                <!-- image -->
                <div class="col-lg-4" v-show="element.type==='image' && isElementInPage(index)">
                  <div class="card mb-3">
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                         v-if="element.type==='image'">

                    <div class="card-body">
                      <input class="form-control-plaintext" type="text" :value="element.title" maxlength="100"
                             @change="updateElementTitle(element.id, $event)">
                      <span class="card-text"><small
                        class="text-muted">{{moment(element.created_at).format('lll')}}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element, $event)"><i
                        class="fas fa-trash"></i></a>
                    </div>
                  </div>
                </div>
              </template>
            </div>

            <b-pagination
              v-model="currentPage"
              :total-rows="totalRow"
              :per-page="perPage"
              first-number
              last-number
              align="center"
            ></b-pagination>
          </div>


          <div class="tab-pane fade" id="v-pills-messages" role="tabpanel" aria-labelledby="v-pills-messages-tab">...
          </div>
          <div class="tab-pane fade" id="v-pills-settings" role="tabpanel" aria-labelledby="v-pills-settings-tab">...
          </div>
        </div>
      </div>
    </div>


  </div>
</template>

<script>
  import bsCustomFileInput from 'bs-custom-file-input';
  import moment from 'moment';

  const LOADING_POST = 0;
  const SAVING_POST = 1;
  const UPLOADING_VIDEO = 2;

  // import { localize } from 'vee-validate';
  export default {
    created() {
      this.LOADING_POST = LOADING_POST;
      this.SAVING_POST = SAVING_POST;
      this.UPLOADING_VIDEO = UPLOADING_VIDEO;
    },
    mounted() {
      bsCustomFileInput.init();
      this.loadPost();
      this.loadElements();

      // localize('zh_TW');
      //debug
      // this.serial = 'a14b32';
    },
    props: {
      showPostEndpoint: String,
      getElementsEndpoint: String,
      updatePostEndpoint: String,
      updateElementEndpoint: String,
      deleteElementEndpoint: String,
      createImageElementEndpoint: String,
      createVideoElementEndpoint: String
    },
    data: function () {
      return {
        loading: {
          LOADING_POSTS: true,
          SAVING_POST: false,
          UPLOADING_VIDEO: false
        },
        uploadingFiles: {},
        post: null,
        isEditing: false,
        postElements: [],
        elements: [],
        stashElement: [],
        playingVideo: null,

        currentPage: 1,
        perPage: 50,
        elementsPagination: [],
        uploadVideoUrl: "https://www.youtube.com/watch?v=j1hft9Wjq9U",

        // Alert
        dismissSecs: 5,
        dismissCountDown: 0,
        alertLevel: 'success',
        alertText: ''

      }
    },
    computed: {
      totalRow: function () {
        return this.elements.length;
      }
    },
    methods: {

      // Alert
      showAlert(text) {
        this.alertText = text;
        this.dismissCountDown = this.dismissSecs;
      },
      dismissAlert() {
        this.dismissCountDown = 0;
      },

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
      loadElements: function (page = 1) {
        const params = {
          per_page: page
        };
        axios.get(this.getElementsEndpoint, {params: params})
          .then(res => {
            this.postElements = res.data;
            let counter = 1;
            _.each(this.postElements.data, (element) => {
              setTimeout(() => {
                this.elements.push(element);
              }, 100 * counter++);
              // this.elements.push(element);
            });

            if (this.postElements.current_page < this.postElements.last_page) {
              this.loadElements(this.postElements(this.postElements.current_page + 1));
            }
          })
      },
      isElementInPage(index) {
        return (this.currentPage - 1) * this.perPage <= index
          && this.currentPage * this.perPage > index;
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
            this.post = res.data.data;
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
          form.append('post_serial', this.post.serial);
          axios.post(this.createImageElementEndpoint, form, {
            onUploadProgress: progressEvent => {
              console.log(progressEvent.loaded / progressEvent.total * 100);
              this.updateProgressBarValue(file, progressEvent);
            }
          })
            .then(res => {
              console.log(res.data);
              this.elements.unshift(res.data.data);
              this.deleteProgressBarValue(file);
            });
        });
      },
      uploadVideo: function () {

        this.uploadLoadingStatus(UPLOADING_VIDEO, true);
        const data = {
          post_serial: this.post.serial,
          url: this.uploadVideoUrl
        };
        axios.post(this.createVideoElementEndpoint, data)
          .then(res => {
            console.log(res.data);
            this.elements.unshift(res.data.data);
          })
          .finally(() => {
            this.uploadLoadingStatus(UPLOADING_VIDEO, false);
          });

      },
      updateElementTitle: function (id, event) {
        console.log(id);
        console.log(event.target.value);
        console.log(this.toSeconds(event.target.value));

        const data = {
          title: event.target.value
        };
        const url = this.updateElementEndpoint.replace('_id', id);
        axios.put(url, data)
          .then((res) => {
            console.log('updateElementTitle');
          })
      },
      updateVideoScope: function (index, element, event) {
        let key = event.target.name;
        let seconds = this.toSeconds(event.target.value);
        if(!Number.isInteger(seconds)){
          return ;
        }
        const data = {
          [key]: seconds
        };
        const url = this.updateElementEndpoint.replace('_id', element.id);
        axios.put(url, data)
          .then((res) => {
            element[key] = seconds;
            this.$set(this.elements, index, element);
          })
      },
      deleteElement: function (element, event) {
        //spin button
        const trashcan = event.target.getElementsByTagName('i')[0];
        const originClass = trashcan.getAttribute('class');
        trashcan.setAttribute('class', 'spinner-border spinner-border-sm');

        const url = this.deleteElementEndpoint.replace('_id', element.id);
        axios.delete(url)
          .then((res) => {
            this.stashElement.push(element);
            const index = _.findIndex(this.elements, {
              id: element.id
            });
            this.$delete(this.elements, index);
            console.log(event.target.name + " deleted");
          })
          .finally(() => {
            trashcan.setAttribute('class', originClass);
          });
      },
      updateProgressBarValue: function (file, progressEvent) {
        let filename = file.name;
        this.$set(this.uploadingFiles, filename, Math.round(progressEvent.loaded / progressEvent.total * 100));
      },
      deleteProgressBarValue: function (file) {
        delete this.uploadingFiles[file.name];
        this.uploadingFiles = Object.assign({}, this.uploadingFiles);
      },
      toTimeFormat: function (seconds) {
        if (seconds === null || seconds === '') {
          return null;
        }
        if (seconds >= 3600) {
          return moment.utc(seconds * 1000).format('HH:mm:ss');
        } else {
          return moment.utc(seconds * 1000).format('mm:ss');
        }
      },
      toSeconds: function (time) {
        let timeGroup = time.match(/^(?:(?:([01]?\d|2[0-3]):)?([0-5]?\d):)?([0-5]?\d)$/);
        return moment.duration({
          seconds: timeGroup[3],
          minutes: timeGroup[2],
          hours: timeGroup[1],
        }).asSeconds();
      },
      getPlayer(element) {
        return _.get(this.$refs, element.id + '.0.player', null);
      },
      clickPlayButton(index, element) {
        this.playingVideo = element.id;
        element.loadedVideo = true;
        this.$set(this.elements, index, element);
      },
      doPlay(element) {
        const player = this.getPlayer(element);
        if (player) {

          player.loadVideoById({
            videoId: element.video_id,
            startSeconds: element.video_start_second,
            endSeconds: element.video_end_second
          });
        }
      }

    }
  }

</script>
