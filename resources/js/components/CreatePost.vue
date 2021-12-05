<template>
  <div class="container-fluid">
    <div class="py-5 text-center">
      <div class="row">
        <h2 class="mb-3">殘酷2選1基本資訊</h2>
        <div class="col-12">
          <label for="title">標題</label>
          <input type="text" class="form-control" id="title" name="title" required>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  export default {
    mounted() {

    },
    props: {
      getPostEndpoint: String,
      createPostEndpoint: String,
      updatePostEndpoint: String,
      createImageElementEndpoint: String,
      createVideoElementEndpoint: String
    },
    data: function () {
      return {
      }
    },
    methods: {

    }
  }

</script>
