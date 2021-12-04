<template>
  <div class="container">

    <div class="row">
      <div class="col-auto"><h1>{{post.title}}</h1></div>
    </div>
    <div class="row mb-2">
      <div class="col-auto"><p>{{post.description}}</p></div>
    </div>
    <div class="row">
      <div class="col-md-6">
        <div class="card">
          <div class="position-relative">
            <img src="https://via.placeholder.com/640x480.png/005544?text=qui" class="card-img-top" alt="">
            <h3 class="position-absolute text-white fixed-top m-3 bg-dark bg-opacity-30 shadow p-3 mb-5 rounded">position-absolute</h3>
          </div>
          <div class="card-body">
            <h5 class="card-title">card-title</h5>
            <p class="card-text">card-text</p>
          </div>
        </div>
      </div>
      <div class="col-md-6">
        <div class="card">
          <div class="position-relative">
            <img src="https://via.placeholder.com/640x480.png/331144?text=quimoiqw" class="card-img-top" alt="">
            <h3 class="position-absolute text-white fixed-top m-3 bg-dark bg-opacity-30 shadow p-3 mb-5 rounded">position-absolute</h3>
          </div>
          <div class="card-body">
            <h5 class="card-title">card-title</h5>
            <p class="card-text">card-text</p>
          </div>
        </div>
      </div>
    </div>


  </div>
</template>

<script>
  export default {
    mounted() {
      this.loadRankData();
    },
    props: {
      postSerial: String,
      showRankEndpoint: String

    },
    data: function () {
      return {
        post: null
      }
    },
    methods: {
      loadRankData: function () {
        axios.get(this.showRankEndpoint)
          .then(res => {
            this.post = res.data.data;
          })
      }
    }
  }

</script>
