<template>
  <!--  Main -->
  <div class="container-fluid">

    <!-- Alert -->
    <div class="position-fixed" style="top: 20px; right: 15px; z-index: 10">
      <span class="cursor-pointer" @click="dismissAlert">
        <b-alert :show="dismissCountDown" @dismissed="dismissCountDown = 0" dismissible fade :variant="alertLevel">
          {{ alertText }}
        </b-alert>
      </span>
    </div>

    <div class="row">

      <!-- tabs -->
      <div class="col-md-2">
        <div class="nav flex-column nav-pills" id="v-pills-tab" role="tablist" aria-orientation="vertical">

          <a class="nav-link active" id="v-pills-post-info-tab" data-toggle="pill" href="#v-pills-post-info" role="tab"
            aria-controls="v-pills-post-info" aria-selected="true">
            <i v-if="isEditing" class="fa-xl fa-solid fa-triangle-exclamation" style="color:red" v-b-tooltip.hover
              :title="$t('Unsaved')"></i>
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-info-circle"></i>
            </span>
            {{ $t('edit_post.tab.info') }}
          </a>
          <a class="nav-link" id="v-pills-elements-tab" data-toggle="pill" href="#v-pills-elements" role="tab"
            aria-controls="v-pills-elements" aria-selected="false">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-photo-video"></i>
            </span>
            {{ $t('edit_post.tab.element') }}
          </a>
          <a class="nav-link" id="v-pills-rank-tab" data-toggle="pill" href="#v-pills-rank" role="tab"
            aria-controls="v-pills-rank" aria-selected="false">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fas fa-trophy"></i>
            </span>
            {{ $t('edit_post.tab.rank') }}
          </a>
          <!-- <a class="nav-link" id="v-pills-comments-tab" data-toggle="pill" href="#v-pills-comments" role="tab"
            aria-controls="v-pills-comments" aria-selected="false" @click="clickCommentTab">
            <span class="d-inline-block text-center" style="width: 30px">
              <i class="fa-solid fa-comment-dots"></i>
            </span>
            {{ $t('edit_post.tab.comments') }}
          </a> -->
        </div>
      </div>

      <div class="col-sm-12 col-md-10">
        <div class="tab-content" id="v-pills-tabContent">

          <!-- tab info -->
          <div class="tab-pane fade show active" id="v-pills-post-info" role="tabpanel"
            aria-labelledby="v-pills-post-info-tab">
            <!-- Loading -->
            <div class="col-9 fa-3x text-center" v-if="loading['LOADING_POST']">
              <i class="fas fa-spinner fa-spin"></i>
            </div>

            <ValidationObserver v-slot="{ invalid }" v-if="post && !loading['LOADING_POST']">
              <!-- Rank基本資訊 -->
              <div class="row">
                <div class="col-6">
                  <h2 class="mt-3 mb-3">{{ $t('edit_post.info.head') }}</h2>
                </div>
                <div class="col-6">
                  <h2 class="mt-3 mb-3">
                    <span class="d-flex justify-content-end">
                      <a class="btn btn-danger mr-3" v-if="!isEditing" :href="playGameRoute" target="_blank">
                        <i class="fas fa-play"></i> {{ $t('edit_post.info.play') }}
                      </a>

                      <button class="btn btn-primary" v-if="!isEditing" @click="clickEdit">
                        <i class="fas fa-edit"></i> {{ $t('edit_post.info.edit') }}
                      </button>

                      <!-- Editing -->
                      <button class="btn btn-secondary mr-3" v-if="isEditing" @click="cancelEdit"
                        :disabled="loading['SAVING_POST']">
                        <i class="fa-solid fa-rectangle-xmark"></i>
                        {{ $t('edit_post.info.cancel_save') }}
                      </button>
                      <button class="btn btn-primary" v-if="isEditing" @click="savePost"
                        :disabled="invalid || loading['SAVING_POST']">
                        <i class="fas fa-save" v-if="!loading['SAVING_POST']"></i>
                        <i class="fas fa-spinner fa-spin" v-if="loading['SAVING_POST']"></i>
                        {{ $t('edit_post.info.save') }}
                      </button>
                    </span>
                  </h2>
                </div>
              </div>
              <form @submit.prevent>
                <!-- 標題 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="title">
                        {{ $t('Title') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <input type="text" class="form-control" id="title" v-model="post.title" required
                          autocomplete="off" :disabled="!isEditing || loading['SAVING_POST']"
                          :maxlength="config.post_title_size">
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <!-- 描述 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg" for="description">
                        {{ $t('Description') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }">
                        <textarea class="form-control" id="description" v-model="post.description" rows="3"
                          style="resize: none" :maxlength="config.post_description_size"
                          aria-describedby="description-help" required :disabled="!isEditing || loading['SAVING_POST']">

                        </textarea>
                        <small id="description-help" class="form-text text-muted">
                          {{ $t('create_game.description.hint') }}
                        </small>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <!-- 標籤 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('Tag') }}
                      </label>
                      <ValidationProvider v-slot="{ errors }">
                        <div class="input-group mb-3" v-if="isEditing">
                          <div class="input-group-prepend">
                            <span class="input-group-text" id="hashtag">#</span>
                          </div>
                          <input list="tagsOptions" type="text" class="form-control" autocomplete="off"
                            :placeholder="$t('edit_post.info.max_hashtag')" maxlength="15" aria-label="hashtag"
                            aria-describedby="hashtag" v-model="tagInput" @keyup="loadTagsOptions">
                          <datalist id="tagsOptions">
                            <option v-for="tag in tagsOptions" :value="tag.name">{{ tag.name }}</option>
                          </datalist>
                          <div class="input-group-append">
                            <button class="btn btn-outline-secondary" type="button" id="hashtag" @click="addTag"
                              :disabled="tags.length >= 5">{{ $t('edit_post.tag.enter') }}</button>
                          </div>
                        </div>
                        <div class="form-group">
                          <span v-for="tag in tags" class="badge badge-secondary mr-1" style="font-size: larger;">
                            {{ tag }}
                            <a v-if="isEditing" class="btn btn-sm btn-light ml-1" @click="removeTag(tag)"
                              @keydown.enter.prevent>
                              <i class="fas fa-times"></i>
                            </a>
                          </span>
                          <span v-if="tags.length === 0" class="text-muted">{{ $t('edit_post.info.no_tag') }}</span>
                        </div>
                      </ValidationProvider>
                    </div>
                  </div>
                </div>
                <!-- 發佈 -->
                <div class="row">
                  <div class="col-12">
                    <div class="form-group">
                      <label class="col-form-label-lg">
                        {{ $t('Visibility') }}
                      </label>
                      <ValidationProvider rules="required" v-slot="{ errors }" v-if="isEditing">
                        <label class="form-control btn btn-outline-dark" for="post-privacy-public">
                          <input type="radio" id="post-privacy-public" v-model="post.policy" value="public"
                            :disabled="loading['SAVING_POST']">

                          {{ $t('Public') }}
                        </label>

                        <label class="form-control btn btn-outline-dark" for="post-privacy-private">
                          <input type="radio" id="post-privacy-private" v-model="post.policy" value="private"
                            :disabled="loading['SAVING_POST']">
                          {{ $t('Private') }}
                        </label>
                        <span class="text-danger">{{ errors[0] }}</span>
                      </ValidationProvider>
                      <div v-else>
                        <input class="form-control" disabled="disabled" :value="$t('post_policy.' + post.policy)">
                        <small class="form-text text-muted" v-if="post.policy === 'public'">{{
        $t('edit_post.at_least_element_number_hint') }}</small>
                      </div>
                    </div>
                  </div>
                </div>
              </form>
              <!-- 建立時間 -->
              <div class="row" v-if="post">
                <div class="col-3">
                  <div class="form-group">
                    <label class="col-form-label-lg">{{ $t('edit_post.info.create_time') }}</label>
                    <input class="form-control" disabled :value="post.created_at | date">
                  </div>
                </div>
              </div>
              <!-- Delete Post Button -->
              <div class="row" v-if="post && isEditing">
                <div class="col-12">
                  <div class="form-group fa-pull-right">
                    <button class="btn btn-danger" @click="deletePost" :disabled="loading['DELETING_POST']">
                      <i class="fas fa-trash-alt"></i> {{ $t('edit_post.info.delete') }}
                    </button>
                  </div>
                </div>
              </div>
            </ValidationObserver>
          </div>

          <!-- tab elements -->
          <div class="tab-pane fade" id="v-pills-elements" role="tabpanel" aria-labelledby="v-pills-elements-tab">
            <!-- upload image from device -->
            <!-- 上傳圖片 -->
            <h2 class="mt-3 mb-3"><i class="fa-regular fa-image"></i>&nbsp;{{ $t('edit_post.upload_image') }}</h2>

            <div class="row">
              <div class="col-12">
                <label for="image-upload"><i class="fa-solid fa-upload"></i>&nbsp;{{ $t('Upload from Local') }}</label>
                <div class="custom-file form-group">
                  <input type="file" class="custom-file-input" id="image-upload" multiple @change="uploadImages">
                  <label class="custom-file-label" for="image-upload">Choose File...</label>
                </div>
              </div>
            </div>
            <!-- upload progress bar -->
            <div class="progress mb-1" v-for="(progress, name) in uploadingFiles">
              <div class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" aria-valuemin="0"
                aria-valuemax="100" :aria-valuenow="progress" :style="{ 'width': progress + '%' }">
                {{ name }}
              </div>
            </div>

            <!-- batch upload -->
            <!-- 從網址上傳 -->
            <h2 class="mt-5 mb-3"><i class="fa-solid fa-photo-film"></i>&nbsp;{{ $t('edit_post.upload_batch') }}</h2>
            <div class="row mt-3">
              <div class="col-12">
                <label><i class="fa-solid fa-link"></i>&nbsp;{{ $t('Upload from URL') }}</label>
                <div class="input-group">
                  <textarea class="form-control" type="text" id="batchCreate" name="batchCreate" rows="5"
                    v-model="batchString" aria-label="https://www.youtube.com/watch?v=dQw4w9WgXcQ"
                    aria-describedby="batchCreateVideo" placeholder="https://www.youtube.com/watch?v=dQw4w9WgXcQ">
                  </textarea>
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" @click="batchUpload"
                      :disabled="loading['BATCH_UPLOADING']">
                      <span v-show="!loading['BATCH_UPLOADING']">{{ $t('edit_post.add_video_button') }}</span>
                      <i v-show="loading['BATCH_UPLOADING']" class="fas fa-spinner fa-spin"></i>
                    </button>
                  </div>
                </div>
                <div class="mt-3">
                  <template>
                    <UploadIcon />
                  </template>
                </div>
              </div>
            </div>

            <!-- edit -->

            <h2 class="mt-5 mb-3"><i class="fa-solid fa-gear"></i>&nbsp;{{ $t('edit_post.edit_media') }}</h2>

            <p>{{ $t('Max :number elements',{ number: config.post_max_element_count }) }}</p>
            <p>{{ $t('total elements', { count: totalRow }) }}</p>

            <nav class="navbar navbar-light bg-light pr-0 justify-content-end">
              <div class="form-inline">
                <input class="form-control mr-sm-2" v-model="filters.title_like" type="search"
                  :placeholder="$t('Search')" aria-label="Search" @change="loadElements(1)">
                <i class="fa-solid fa-magnifying-glass" v-if="!filters.title_like"></i>
                <i class="fas fa-filter" v-if="filters.title_like"></i>
              </div>
            </nav>

            <div class="row">
              <template v-for="(element, index) in elements.data">
                <!-- show video card -->
                <div class="col-lg-4 col-md-6" v-show="isVideoSource(element)">
                  <!-- youtube source -->
                  <div class="card mb-3" v-show="isYoutubeSource(element)">
                    <youtube v-if="isYoutubeSource(element) && element.loadedVideo" width="100%" height="270"
                      :ref="element.id" @ready="doPlay(element)" :player-vars="{
                      controls: 1,
                      autoplay: 0,
                      start: element.video_start_second,
                      end: element.video_end_second,
                      rel: 0,
                      origin: host
                    }"></youtube>
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                      v-show="isYoutubeSource(element) && !element.loadedVideo">
                    <!-- youtube video editor -->
                    <div class="card-body">
                      <!--title edit-->
                      <!-- <input class="form-control-plaintext bg-light cursor-pointer p-2" type="text" :value="element.title"
                        :maxlength="config.element_title_size" @change="updateElementTitle(element.id, $event)"> -->
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <!--play time range-->
                      <div class="row mb-3">
                        <div class="col-10">
                          <div class="input-group">
                            <div class="input-group-prepend d-lg-none d-xl-block">
                              <span class="input-group-text">{{ $t('edit_post.video_range') }}</span>
                            </div>
                            <input type="text" class="form-control" name="video_start_second" placeholder="0:00"
                              aria-label="start" @change="updateVideoScope(index, element, $event)"
                              :value="toTimeFormat(element.video_start_second)">
                            <div class="input-group-prepend"><span class="input-group-text">~</span></div>
                            <input type="text" class="form-control" name="video_end_second"
                              :placeholder="toTimeFormat(element.video_duration_second)" aria-label="end"
                              :value="toTimeFormat(element.video_end_second)"
                              @change="updateVideoScope(index, element, $event)">
                          </div>
                        </div>
                        <!--play button-->
                        <div class="col-2">
                          <a class="btn btn-danger float-right" @click="clickPlayButton(index, element)">
                            <i class="fas fa-play-circle"></i>
                          </a>
                        </div>
                      </div>
                      <!--create time-->
                      <span class="card-text"><small class="text-muted">{{ element.created_at | datetime
                          }}</small></span>
                      <!--delete button-->
                      <a class="btn btn-danger float-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                    </div>
                  </div>

                  <!-- simple video source -->
                  <div class="card mb-3" v-show="!isYoutubeSource(element)">
                    <!-- load the video player -->
                    <video width="100%" height="270" loop autoplay muted playsinline :src="element.thumb_url"></video>
                    <!-- editor -->
                    <div class="card-body">
                      <input class="form-control-plaintext bg-light cursor-pointer mb-2 p-2" type="text"
                        :value="element.title" :maxlength="config.element_title_size"
                        @change="updateElementTitle(element.id, $event)">
                      <span class="card-text"><small class="text-muted">{{ element.created_at | datetime
                          }}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                    </div>
                  </div>

                </div>
                <!-- image player -->
                <div class="col-lg-4 col-md-6" v-if="element.type === 'image'">
                  <div class="card mb-3">
                    <img :src="element.thumb_url" class="card-img-top" :alt="element.title"
                      v-if="element.type === 'image'">

                    <div class="card-body">
                      <textarea class="form-control-plaintext bg-light cursor-pointer p-2 mb-2" v-model="element.title"
                        :maxlength="config.element_title_size" rows="4" style="resize: none;"
                        @change="updateElementTitle(element.id, $event)"></textarea>
                      <span class="card-text"><small class="text-muted">{{ element.created_at | datetime
                          }}</small></span>
                      <a class="btn btn-danger float-right" @click="deleteElement(element)">
                        <i class="fas fa-trash" v-if="!isDeleting(element)"></i>
                        <i class="spinner-border spinner-border-sm" v-if="isDeleting(element)"></i>
                      </a>
                    </div>
                  </div>
                </div>
              </template>
            </div>

            <b-pagination v-model="currentPage" v-if="elements.meta.last_page > 1" :total-rows="totalRow"
              :per-page="perPage" first-number last-number @change="handleElementPageChange"
              align="center"></b-pagination>
          </div>

          <!-- tab rank -->
          <div class="tab-pane fade" id="v-pills-rank" role="tabpanel" aria-labelledby="v-pills-rank-tab">
            <div class="row" v-if="!loading['LOADING_POST']">
              <div class="col-3">
                <div class="form-group">
                  <label>{{ $t('edit_post.rank.game_plays') }}</label>
                  <input class="form-control" disabled :value="post.play_count">
                </div>
              </div>
            </div>
            <table class="table table-hover" style="table-layout: fixed">
              <thead>
                <tr>
                  <th scope="col">#</th>
                  <th scope="col" style="width: 70%"></th>
                  <th scope="col">{{ $t('edit_post.rank.win_at_final') }}</th>
                  <th scope="col">{{ $t('edit_post.rank.win_rate') }}</th>
                </tr>
              </thead>
              <tbody v-if="loading['LOADING_RANK']">
                <tr>
                  <td colspan="4">
                    <div class="fa-3x text-center">
                      <i class="fas fa-spinner fa-spin"></i>
                    </div>
                  </td>
                </tr>
              </tbody>
              <tbody v-if="rank.rankReportData && !loading['LOADING_RANK']">
                <tr v-for="(rank, index) in rank.rankReportData.data">
                  <th scope="row">{{ rank.rank }}</th>
                  <td style="overflow: scroll">
                    <div>
                      <img :src="rank.element.thumb_url" height="300px" :alt="rank.element.title">

                      <a v-if="rank.element.type === 'video'" :href="rank.element.thumb_url" target="_blank">
                        <p>{{ rank.element.title }}</p>
                      </a>
                      <p v-if="rank.element.type === 'image'">{{ rank.element.title }}</p>
                    </div>
                  </td>
                  <td>{{ rank.final_win_rate | percent }}</td>
                  <td>{{ rank.win_rate | percent }}</td>
                </tr>
              </tbody>
            </table>

            <div class="row">
              <div class="col-12" v-if="rank.rankReportData">
                <b-pagination v-model="rank.currentPage" :total-rows="rank.rankReportData.meta.total"
                  :per-page="rank.rankReportData.meta.per_page" first-number last-number @change="handleRankPageChange"
                  align="center"></b-pagination>
              </div>
            </div>
          </div>

          <!-- tab comments -->
          <!-- <div class="tab-pane fade" id="v-pills-comments" role="tabpanel" aria-labelledby="v-pills-comments-tab">
            <div class="row">
              <div class="col-12">
                <div class="card">
                  <div v-if="loading['COMMENTS']">
                    <div class="text-center">
                      <i class="fa-3x fas fa-spinner fa-spin"></i>
                    </div>
                  </div>

                  <div class="card-body">
                    <div v-for="comment in comments.data">
                      <div class="d-flex justify-content-between w-100">
                        
                        <div class="avatar-container">
                          <div class="avatar">
                            <img v-if="comment.avatar_url" :src="comment.avatar_url" :alt="comment.nickname">
                            <img v-else :src="defaultAvatarUrl" :alt="comment.nickname">
                          </div>
                        </div>
                        <div class="comment-container">
                          
                          <div class="d-flex justify-content-between">
                            <span class="text-black-50 font-size-large" style="overflow-wrap:anywhere"><small>{{comment.nickname}}</small></span>
                            <div class="ml-auto">
                              <div class="text-align-end">
                                <div class="dropdown">
                                  <span href="#" role="button" id="reportDropdown" data-toggle="dropdown"
                                    aria-haspopup="true" aria-expanded="false">
                                    <i class="fa-xl fa-solid fa-ellipsis-vertical cursor-pointer text-center"
                                      style="width: 20px"></i>
                                  </span>

                                  <div class="dropdown-menu dropdown-menu-right" aria-labelledby="reportDropdown">
                                    <a class="dropdown-item"  href="#"><i
                                        class="fa-solid fa-triangle-exclamation"></i>&nbsp;REPORT</a>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </div>
                          
                          <div class="overflow-hidden">
                            <template v-for="champion in comment.champions">
                              <span class="badge badge-secondary mr-1 mb-1" :title="champion">{{ champion }}</span>
                            </template>
                          </div>
                          
                          <p class="break-all white-space-pre-line overflow-scroll" style="max-height: 200px;">
                            {{ comment.content }}</p>
                          
                          <div class="text-right text-muted">
                            {{ comment.created_at | datetime }}
                          </div>
                        </div>
                      </div>
                    <hr class="my-4">
                    </div>

                    <div v-if="comments.data.length == 0" class="text-center">
                      <p>{{ $t('No comments') }}</p>
                    </div>
                  </div>
                  
                  
                  <b-pagination v-if="comments.meta.last_page > 1" v-model="comments.meta.current_page" :total-rows="comments.meta.total"
                    :per-page="comments.meta.per_page" @input="changeCommentPage" class="justify-content-center"></b-pagination>
                </div>
              </div>
            </div>
          </div> -->
          
        </div>
      </div>
    </div>
  </div>
  </div>
</template>

<script>
import bsCustomFileInput from 'bs-custom-file-input';
import moment from 'moment';
import Swal from 'sweetalert2';
import UploadIcon from './partials/UploadIcon.vue';
import { config } from 'vue/types/umd';


export default {
  mounted() {
    bsCustomFileInput.init();
    this.loadPost();
    this.loadElements();
    this.loadRankReport();
    this.loadTagsOptions();
    this.host = window.location.origin;
  },
  props: {
    config: Object,
    showPostEndpoint: String,
    playGameRoute: String,
    getElementsEndpoint: String,
    getRankEndpoint: String,
    updatePostEndpoint: String,
    updateElementEndpoint: String,
    deleteElementEndpoint: String,
    createImageElementEndpoint: String,
    batchCreateEndpoint: String,
    getTagsOptionsEndpoint: String,
    getCommentsEndpoint: String,
    defaultAvatarUrl: String
  },
  data: function () {
    return {
      host: '',
      loading: {
        LOADING_POST: true,
        SAVING_POST: false,
        UPLOADING_YOUTUBE_VIDEO: false,
        LOADING_RANK: false,
        UPLOADING_IMAGE: false,
        UPLOADING_VIDEO_URL: false,
        BATCH_UPLOADING: false,
        DELETING_POST: false,
        COMMENTS: false,
      },
      uploadingFiles: {},
      post: null,
      keep_post: null,
      isEditing: false,
      elements: {
        data: [],
        meta: {
          last_page: 1,
        }
      },
      playingVideo: null,
      deletingElement: [],

      currentPage: 1,
      perPage: 50,
      youtubeUrl: "",
      videoUrl: "",
      batchString: "",
      imageUrl: "",

      // Alert
      dismissCountDown: 0,
      alertLevel: 'success',
      alertText: '',

      // search elements
      filters: {
        title_like: null
      },

      //rank
      rank: {},

      //tags
      tagInput: "",
      oldTagInput: "",
      tags: [],
      keep_tags: [],
      tagsOptions: [],
      tagLocalStash: {},

      //comment
      comments: {
        data: [],
        meta: {
          last_page: 1,
        }
      },

    }
  },
  components: {
    UploadIcon
  },
  computed: {
    totalRow: function () {
      if (this.elements && this.elements.meta) {
        return this.elements.meta.total;
      }
      return 0;
    },

  },
  methods: {

    /** Alert **/
    showAlert(text, level = 'success', dismissSecs = 5) {
      this.alertText = text;
      this.alertLevel = level;
      this.dismissCountDown = dismissSecs;
    },
    dismissAlert() {
      this.dismissCountDown = 0;
    },

    /** Loading **/
    uploadLoadingStatus(key, status) {
      this.$set(this.loading, key, status);
    },

    /** Post **/
    loadPost: function () {
      this.uploadLoadingStatus('LOADING_POST', true);
      const url = this.showPostEndpoint;
      axios.get(url)
        .then(res => {
          this.post = res.data.data;
          this.tags = res.data.data.tags;
          this.keep_post = _.cloneDeep(res.data.data);
          this.keep_tags = _.cloneDeep(res.data.data.tags);
        })
        .finally(() => {
          this.uploadLoadingStatus('LOADING_POST', false);
        });

    },
    clickEdit: function () {
      this.isEditing = true;
    },
    cancelEdit: function () {
      Swal.fire({
        title: this.$t("Are you sure?"),
        text: this.$t("You will lose all unsaved changes!"),
        icon: "warning",
        showCancelButton: true,
      })
        .then((willDelete) => {
          if (willDelete.isConfirmed) {
            this.isEditing = false;
            this.post = _.cloneDeep(this.keep_post);
            this.tags = _.cloneDeep(this.keep_tags);
          }
        });
    },
    savePost: function () {
      this.uploadLoadingStatus('SAVING_POST', true);
      const data = {
        title: this.post.title,
        description: this.post.description,
        policy: {
          access_policy: this.post.policy
        },
        tags: this.tags
      };

      axios.put(this.updatePostEndpoint, data)
        .then(res => {
          this.post = res.data.data;
          this.tags = res.data.data.tags;
          this.isEditing = false;
        })
        .finally(() => {
          this.uploadLoadingStatus('SAVING_POST', false);
        });
    },
    deletePost: function () {
      this.uploadLoadingStatus('DELETING_POST', true);
      Swal.fire({
        title: this.$t('Enter Password'),
        input: 'password',
        inputAttributes: {
          autocapitalize: 'off'
        },
        showCancelButton: true,
        confirmButtonText: this.$t('Delete'),
        cancelButtonText: this.$t('Cancel'),
        showLoaderOnConfirm: true,
        preConfirm: (password) => {
          return axios.delete(this.updatePostEndpoint, { data: { password: password } })
            .then(res => {
            })
            .catch(error => {
              Swal.showValidationMessage(this.$t(`Request failed`));
            });
        },
        allowOutsideClick: () => !Swal.isLoading()
      }).then((result) => {
        if (result.isConfirmed) {
          this.uploadLoadingStatus('LOADING_POST', true);
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            toast: true,
            title: this.$t('Deleted!'),
            text: this.$t('Your post has been deleted.'),
            icon: 'success',
            timer: 3000
          }).then(res => {
            window.location.href = '/account/post';
          });
        }
      }).finally(() => {
        this.uploadLoadingStatus('DELETING_POST', false);
      });
    },

    /** Elements **/
    loadElements: function (page = 1) {
      const pagination = {
        page: page,
      };
      const params = {
        ...pagination,
        ...{ filter: this.filters }
      }
      axios.get(this.getElementsEndpoint, { params: params })
        .then(res => {
          this.elements = res.data;
        })
    },
    updateElementTitle: function (id, event) {
      const data = {
        title: event.target.value
      };
      const url = this.updateElementEndpoint.replace('_id', id);
      axios.put(url, data)
        .then((res) => {
          Swal.fire({
            position: "top-end",
            showConfirmButton: false,
            title: this.$t("Updated!"),
            toast: true,
            text: this.$t("The element has been updated."),
            icon: "success",
            timer: 3000
          });
        })
    },
    deleteElement: function (element) {
      Swal.fire({
        title: this.$t("Are you sure?"),
        text: this.$t("Once deleted, you will not be able to recover this element!"),
        icon: "warning",
        showCancelButton: true,
      })
        .then((willDelete) => {
          if (willDelete.isConfirmed) {
            this.pushDeleting(element);

            const url = this.deleteElementEndpoint.replace('_id', element.id);
            axios.delete(url)
              .then((res) => {
                const index = _.findIndex(this.elements.data, {
                  id: element.id
                });
                this.$delete(this.elements.data, index);
                this.elements.meta.total--;
                Swal.fire({
                  position: "top-end",
                  title: this.$t("Deleted!"),
                  toast: true,
                  text: this.$t("The element has been deleted."),
                  showConfirmButton: false,
                  icon: "success",
                  timer: 3000
                });
              })
              .finally(() => {
                this.removeDeleting(element);
              });
          }
        });
    },
    pushDeleting(element) {
      this.deletingElement.push(element.id);
    },
    removeDeleting(element) {
      _.remove(this.deletingElement, (v) => {
        return v === element.id;
      });
    },
    isDeleting: function (element) {
      return this.deletingElement.includes(element.id);
    },

    /** Tag */
    addTag: function () {
      const tag = this.tagInput.trim().replace("#", "");
      if (tag.length > 20) {
        tag = tag.substring(0, 20);
      }
      if (this.tags.length >= 5) {
        Swal.fire({
          position: "top-end",
          showConfirmButton: false,
          title: this.$t("Maximum number of tags reached"),
          toast: true,
          icon: "warning",
          timer: 3000
        });
        return;
      }

      if (this.tags.includes(tag)) {
        Swal.fire({
          position: "top-end",
          showConfirmButton: false,
          title: this.$t("Hashtag already exists"),
          toast: true,
          icon: "warning",
          timer: 3000
        });
        return;
      }

      if (tag) {
        this.tags.push(tag);
        this.tagInput = "";
        this.loadTagsOptions();
      }
    },
    removeTag: function (tag) {
      _.remove(this.tags, (v) => {
        return v === tag;
      });
      this.tags = Object.assign([], this.tags);
    },
    loadTagsOptions: function () {
      if (this.tagInput === this.oldTagInput && this.tagInput !== "") {
        return;
      }

      if (this.tagLocalStash[this.tagInput]) {
        this.tagsOptions = this.tagLocalStash[this.tagInput];
        return;
      }

      if (this.tagInputTimeout) {
        clearTimeout(this.tagInputTimeout);
      }

      this.tagInputTimeout = setTimeout(() => {
        const params = {
          prompt: this.tagInput
        };

        axios.get(this.getTagsOptionsEndpoint, { params: params })
          .then(res => {
            this.tagsOptions = res.data;
            this.oldTagInput = this.tagInput;
            this.tagLocalStash[this.tagInput] = res.data;
            const tagLocalStashLength = Object.keys(this.tagLocalStash).length;
            if (tagLocalStashLength > 30) {
              delete this.tagLocalStash[Object.keys(this.tagLocalStash)[0]];
            }
          });
      }, 500);
    },

    /** Image **/
    uploadImages: function (event) {
      Array.from(event.target.files).forEach(file => {
        let form = new FormData();
        form.append('file', file);
        form.append('post_serial', this.post.serial);
        axios.post(this.createImageElementEndpoint, form, {
          onUploadProgress: progressEvent => {
            this.updateProgressBarValue(file, progressEvent);
          }
        })
          .then(res => {
            this.elements.data.push(res.data.data);
            this.elements.meta.total++;
            this.deleteProgressBarValue(file);
            Swal.fire({
              position: "top-end",
              showConfirmButton: false,
              title: this.$t("Uploaded!"),
              toast: true,
              text: this.$t("The image has been uploaded."),
              icon: "success",
              timer: 3000
            });
          })
          .catch((err) => {
            this.showAlert(err.response.data.message, 'danger');
          });
      });
    },

    /** Video **/
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    batchUpload: function () {
      if (this.loading['BATCH_UPLOADING']) {
        return;
      }
      this.uploadLoadingStatus('BATCH_UPLOADING', true);
      const data = {
        post_serial: this.post.serial,
        url: this.batchString
      };
      let tempRollbackData = this.batchString;
      this.batchString = "";
      axios.post(this.batchCreateEndpoint, data)
        .then(res => {
          let waittime = 0;
          _.forEach(res.data.data, (data) => {
            // this.elements.meta.total++;
            // if (this.totalRow < this.perPage) {
            //   this.elements.data.push(data);
            // }
            setTimeout(() => {
              this.dismissAlert();
              this.showAlert(data.title, 'success');
            }, waittime);
            waittime += 500;
          });
          this.loadElements(this.currentPage);
        })
        .catch((err) => {
          console.log(err.response.data);
          let errorUrl = '';
          if (err.response.data.data.error_url) {
            errorUrl = err.response.data.data.error_url + " ";
          }
          if (err.response.data.data.elements) {
            _.forEach(err.response.data.data.elements, (data) => {
              tempRollbackData = tempRollbackData.replace(data.source_url + ',', '')
              tempRollbackData = tempRollbackData.replace(data.source_url, '')
              this.elements.push(data);
            });
          }
          this.showAlert(errorUrl + err.response.data.message, 'danger');
          this.batchString = tempRollbackData.trim();
        })
        .finally(() => {
          this.uploadLoadingStatus('BATCH_UPLOADING', false);
        });
    },
    updateVideoScope: function (index, element, event) {
      let key = event.target.name;
      let seconds = this.toSeconds(event.target.value);
      if (!Number.isInteger(seconds)) {
        return;
      }
      const data = {
        [key]: seconds
      };
      const url = this.updateElementEndpoint.replace('_id', element.id);
      axios.put(url, data)
        .then((res) => {
          element[key] = seconds;
          this.$set(this.elements.data, index, element);
        })
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
    handleElementPageChange: function (page) {
      this.loadElements(page);
    },

    /** Player **/
    getPlayer(element) {
      return _.get(this.$refs, element.id + '.0.player', null);
    },
    clickPlayButton(index, element) {
      this.playingVideo = element.id;
      element.loadedVideo = true;
      this.$set(this.elements.data, index, element);

      this.doPlay(element);
    },
    doPlay(element) {
      const player = this.getPlayer(element);
      if (player) {
        window.player = player;
        player.loadVideoById({
          videoId: element.video_id,
          startSeconds: element.video_start_second,
          endSeconds: element.video_end_second
        });
      }
    },

    /** Rank **/
    loadRankReport: function (page = 1) {
      this.uploadLoadingStatus('LOADING_RANK', true);
      const filter = {
        'page': page
      };
      axios.get(this.getRankEndpoint, { params: filter })
        .then(res => {
          this.$set(this.rank, 'rankReportData', res.data);
          this.$set(this.rank, 'currentPage', res.data.meta.current_page);
        })
        .finally(() => {
          this.uploadLoadingStatus('LOADING_RANK', false);
        });
    },
    handleRankPageChange: function (page) {
      this.loadRankReport(page);
    },
    clickCommentTab: function () {
      if (this.comments.data.length === 0) {
        this.loadComments();
      }
    },
    loadComments: function () {
      this.uploadLoadingStatus('COMMENTS', true);
      axios.get(this.getCommentsEndpoint)
        .then(res => {
          this.comments = res.data;
        })
        .finally(() => {
          this.uploadLoadingStatus('COMMENTS', false);
        });
    },
    changeCommentPage: function (page) {
      this.loadComments(page);
    },
  }
}

</script>
