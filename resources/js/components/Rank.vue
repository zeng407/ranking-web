<template>
  <!--  Main -->
  <div class="container-fluid">

    <table class="table table-hover">
      <thead>
      <tr>
        <th scope="col">#</th>
        <th scope="col"></th>
        <th scope="col">決賽勝率</th>
        <th scope="col">2選1勝率</th>
      </tr>
      </thead>
      <tbody v-if="rankReportData">
      <tr v-for="(rank, index) in rankReportData.data">
        <th scope="row">{{ (index+1) + (currentPage-1)*rankReportData.meta.per_page}}</th>
        <td>
          <div>
            <img :src="rank.element.thumb_url" height="300px" alt="rank.element.title">

            <a v-if="rank.element.type === 'video'" :href="rank.element.source_url" target="_blank">
              <p>{{ rank.element.title }}</p>
            </a>
            <p v-if="rank.element.type === 'image'">{{ rank.element.title }}</p>
          </div>
        </td>
        <td>{{toPercentString(rank.final_win_rate)}}</td>
        <td>{{toPercentString(rank.win_rate)}}</td>
      </tr>
      </tbody>
    </table>

    <div class="row">
      <div class="col-12">
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


  </div>
</template>

<script>
  export default {
    mounted() {
      this.loadRankData();
      this.loadRankReport();
    },
    props: {
      postSerial: String,
      getRankEndpoint: String,
      getRankReportEndpoint: String,

    },
    data: function () {
      return {
        rankInfo: null,
        rankReportData: null,
        currentPage: 1,
      }
    },
    methods: {
      loadRankData: function () {
        axios.get(this.getRankEndpoint)
          .then(res => {
            this.rankInfo = res.data;
          });
      },
      loadRankReport: function (page = 1){
        const filter = {
          'page': page
        };
        axios.get(this.getRankReportEndpoint, {params: filter})
          .then(res => {
            this.rankReportData = res.data;
            this.currentPage = res.data.meta.current_page;
          });
      },
      handlePageChange: function(page) {
        this.loadRankReport(page);
      },
      toPercentString: function (value) {
        if (value) {
          return value + '%';
        }
        return null;
      }

    }
  }

</script>
