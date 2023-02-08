<template>
  <!--  Main -->
  <div class="container">

    <div class="fa-3x text-center" v-if="isLoading">
      <i class="fas fa-spinner fa-spin"></i>
    </div>
    <div v-if="rankInfo">
      <h2>{{ rankInfo.data.title }}</h2>
      <p>{{ rankInfo.data.description }}</p>
    </div>
    <b-tabs content-class="mt-3" v-if="!isLoading">
      <b-tab title="我的排名" v-if="gameResult">
        <div class="card my-1">
          <div class="card-body text-center" style="height: 387px">
            <youtube v-if="isYoutubeSource(gameResult.winner)" width="100%" height="270"
                     :ref="gameResult.winner.id"
                     :videoId="gameResult.winner.video_id"
                     :player-vars="{
                            controls:1,
                            autoplay:0,
                            start:gameResult.winner.video_start_second,
                            rel:0,
                            origin: host
                            }"
            ></youtube>
            <video v-else-if="isVideoSource(gameResult.winner)" width="100%" height="270" loop autoplay muted
                   playsinline :src="gameResult.winner.source_url"></video>
            <img v-else-if="isImageSource(gameResult.winner)" :src="gameResult.winner.thumb_url" height="270"
                 class=""
                 alt="gameResult.winner.title">
            <div class="d-flex flex-column align-items-start">
              <div class="align-self-center">{{ gameResult.winner.title }}</div>
              <div class="align-self-end">
                我的排名：1 <br>
                全體排名：{{ gameResult.winner_rank ? gameResult.winner_rank : '無' }}
              </div>
            </div>
          </div>
        </div>
        <div class="card my-1" v-for="(rank, index) in gameResult.data">
          <div class="card-body text-center" style="height: 387px">
            <youtube v-if="isYoutubeSource(rank.loser)" width="100%" height="270"
                     :ref="rank.loser.id"
                     :videoId="rank.loser.video_id"
                     :player-vars="{
                            controls: 1,
                            autoplay: 0,
                            start: rank.loser.video_start_second,
                            end: rank.loser.video_end_second,
                            rel: 0,
                            origin: host
                            }"
            ></youtube>
            <video v-else-if="isVideoSource(rank.loser)" width="100%" height="270" loop autoplay muted playsinline
                   :src="rank.loser.source_url"></video>
            <img v-else-if="isImageSource(rank.loser)" :src="rank.loser.thumb_url" height="300px"
                 alt="rank.element.title">
            <div class="d-flex flex-column align-items-start">
              <div class="align-self-center">{{ rank.loser.title }}</div>
              <div class="align-self-end">
                我的排名：{{ index + 2 }}<br>
                全體排名：{{ rank.rank ? rank.rank : '無' }}
              </div>
            </div>
          </div>
        </div>
      </b-tab>
      <b-tab title="全體排名">
        <div v-if="rankReportData && !loadingPage" class="card my-1" v-for="(rank, index) in rankReportData.data">
          <div class="card-body text-center" style="height: 387px">
            <youtube v-if="isYoutubeSource(rank.element)" width="100%" height="270"
                     :ref="rank.element.id"
                     :videoId="rank.element.video_id"
                     :player-vars="{
                              controls:1,
                              autoplay:0,
                              start: rank.element.video_start_second,
                              end:rank.element.video_end_second,
                              rel: 0,
                              origin: host
                              }"
            ></youtube>
            <video v-else-if="isVideoSource(rank.element)" width="100%" height="270" loop autoplay muted playsinline
                   :src="rank.element.source_url"></video>
            <img v-else-if="isImageSource(rank.element)" :src="rank.element.thumb_url" height="270"
                 alt="rank.element.title">
            <div class="d-flex flex-column align-items-start">
              <div class="align-self-center">{{ rank.element.title }}</div>
              <div class="align-self-end">
                <span>#{{ rank.rank }}</span><br>
                <span v-if="rank.final_win_rate"> {{ $t('edit_post.rank.win_at_final') }}：{{rank.final_win_rate | percent }}<br></span>
                <span v-if="rank.win_rate>0"> {{ $t('edit_post.rank.win_rate') }}：{{ rank.win_rate | percent }}</span>
                <span v-else> {{ $t('edit_post.rank.win_rate') }}：{{ '0' | percent }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="row">
          <div class="col-12" v-if="rankReportData">
            <b-pagination
              v-model="currentPage"
              :total-rows="rankReportData.meta.total"
              :per-page="rankReportData.meta.per_page"
              first-number
              last-number
              @change="handlePageChange"
              align="center"
            ></b-pagination>
          </div>
        </div>
      </b-tab>
    </b-tabs>


  </div>
</template>

<script>
export default {
  mounted() {
    this.loadGameResult();
    this.loadRankReport();
    this.loadRankData();
    this.host = window.location.origin;
  },
  props: {
    postSerial: String,
    getRankEndpoint: String,
    getRankReportEndpoint: String,
    getGameResultEndpoint: String

  },
  computed: {
    isLoading: function () {
      return this.loadingGameResult || this.loadingGameReport;
    },
  },
  data: function () {
    return {
      host: '',
      currentTab: '',
      rankInfo: null,
      rankReportData: {
        data: {},
        meta: {}
      },
      currentPage: 1,
      gameResult: null,
      loadingGameResult: true,
      loadingGameReport: true,
      loadingPage: true
    }
  },
  methods: {
    loadRankData: function () {
      axios.get(this.getRankEndpoint)
        .then(res => {
          this.rankInfo = res.data;
        });
    },
    loadGameResult: function () {
      const urlParams = new URLSearchParams(window.location.search);
      if (!urlParams.has('g')) {
        this.loadingGameResult = false;
        this.currentTab = '1'
        return;
      }
      this.currentTab = '0';
      const api = this.getGameResultEndpoint.replace('_serial', urlParams.get('g'));
      axios.get(api)
        .then(res => {
          this.gameResult = res.data;
        })
        .finally(() => {
          this.loadingGameResult = false;
        });
    },
    loadRankReport: function (page = 1) {
      this.loadingPage = true;
      const filter = {
        'page': page
      };
      axios.get(this.getRankReportEndpoint, {params: filter})
        .then(res => {
          this.rankReportData = res.data;
          this.currentPage = res.data.meta.current_page;
        })
        .finally(() => {
          this.loadingGameReport = false;
          this.loadingPage = false;
        });
    },
    handlePageChange: function (page) {
      this.loadRankReport(page);
    },
    isYoutubeSource: function (element) {
      return element.type === 'video' && element.video_source === 'youtube';
    },
    isVideoSource: function (element) {
      return element.type === 'video';
    },
    isImageSource: function (element) {
      return element.type === 'image';
    },
    getPlayer(element) {
      return _.get(this.$refs, element.id + '.0.player', null);
    },
    doPlay(element) {
      const player = this.getPlayer(element);
      if (player) {
        window.player = player;
        player.cueVideoById({
          videoId: element.video_id,
          startSeconds: element.video_start_second,
          endSeconds: element.video_end_second
        });
      }
    }
  }
}

</script>
