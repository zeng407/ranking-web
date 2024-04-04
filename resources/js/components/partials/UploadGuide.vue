<template>
  <div>
    <a href="#" data-toggle="modal" data-target="#uploadGuide">{{ $t('Upload guide') }}<i class="fa-solid fa-circle-info"></i></a>
    <!-- Modal for upload guide -->
    <!-- Modal -->
    <div class="modal fade" id="uploadGuide" data-keyboard="false" tabindex="-1" aria-labelledby="uploadGuidePanel"
      aria-hidden="true">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <ul class="nav nav-pills mb-3" id="pills-tab" role="tablist">
              <li class="nav-item" role="presentation">
                <button class="nav-link active" id="pills-youtube-tab" data-toggle="pill" data-target="#pills-youtube"
                  type="button" role="tab" aria-controls="pills-youtube" aria-selected="true" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain" alt="yotube logo" src="/storage/youtube-icon.png">
                </button>
              </li>
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-imgur-tab" data-toggle="pill" data-target="#pills-imgur"
                  type="button" role="tab" aria-controls="pills-imgur" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain;" alt="imgur logo" src="/storage/imgur.svg">
                </button>
              </li>
              <!-- <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-giphy-tab" data-toggle="pill" data-target="#pills-giphy"
                  type="button" role="tab" aria-controls="pills-giphy" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain" alt="giphy logo" src="/storage/giphy-logo-vector.svg"></button>
              </li> -->
              <li class="nav-item" role="presentation">
                <button class="nav-link" id="pills-images-tab" data-toggle="pill" data-target="#pills-images"
                  type="button" role="tab" aria-controls="pills-images" aria-selected="false" style="width: 70px;">
                  <img style="height: 30px; width: 100%; object-fit: contain" alt="images" src="/storage/image-icon.png">
                </button>
              </li>
            </ul>
          </div>
          <div class="modal-body">
            <div class="tab-content" id="pills-tabContent">
              <div class="tab-pane fade show active" id="pills-youtube" role="tabpanel" aria-labelledby="pills-youtube-tab">
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>https://www.youtube.com/watch?v=xxxxx</li>
                  <li>https://www.youtube.com/watch/xxxxx</li>
                  <li>https://youtu.be/xxxxx</li>
                </ul>
                <h5>{{ $t('Import clip') }}</h5>
                <div class="alert alert-dark">
                  {{ youtubeEmbedSample }}
                </div>
              </div>
              <div class="tab-pane fade" id="pills-imgur" role="tabpanel" aria-labelledby="pills-imgur-tab">
                <h5>{{$t('Support formats')}}:</h5>
                <ul>
                  <li>https://imgur.com/gallery/xxxxx</li>
                  <li>https://imgur.com/a/xxxxx</li>
                  <li>https://imgur.com/xxxxx</li>
                  <li>https://i.imgur.com/xxxxx.jpg</li>
                  <li>https://i.imgur.com/xxxxx.png</li>
                  <li>https://i.imgur.com/xxxxx.gif</li>
                </ul>

              </div>
              <!-- <div class="tab-pane fade" id="pills-giphy" role="tabpanel" aria-labelledby="pills-giphy-tab">giphy</div> -->
              <div class="tab-pane fade" id="pills-images" role="tabpanel" aria-labelledby="pills-images-tab">
                <ul>
                  <li>https://exmaple.com/xxxx.gif</li>
                  <li>https://exmaple.com/xxxx.jpg</li>
                  <li>https://exmaple.com/xxxx.png</li>
                  <li>https://exmaple.com/xxxx.mp4</li>
                </ul>
              </div>

            </div>
          </div>


          <div class="modal-footer mb-sm-0 mb-4">
            <button type="button" class="btn btn-secondary" data-dismiss="modal">{{ $t('Close') }}</button>
          </div>

        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      youtubeEmbedSample: '<iframe src="https://www.youtube.com/embed/xxxxx"  ... ></iframe>'
    };
  },
  methods: {
    showGuide() {
      $('#uploadGuide').modal('show');
    },
  },
};
</script>