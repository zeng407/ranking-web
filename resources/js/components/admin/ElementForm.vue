<template>
  <div class="row">
    <div class="col-3" v-for="element in elements">
      <div class="card">
        <div class="card-body">
          <template v-if="element.type === 'image'">
            <img :src="getThumbUrl(element)" class="img-fluid">
            <i class="fas fa-image"></i>
          </template>
          <template v-if="element.type === 'video'">
            <a target="_blank" :href="element.source_url">
              <img :src="getThumbUrl(element)" class="img-fluid">
            </a>
            <i class="fas fa-video"></i>
          </template>
          <input class="form-control mb-3" type="text" name="title" v-model="titles[element.id]">
          <div class="row">
            <div class="col-8">
              <button class="form-control btn btn-sm btn-outline-danger" type="submit"
                @click="onclickUpdateElement(element.id)">更新</button>
            </div>
            <div class="col-4">
              <button class="form-control btn btn-sm btn-outline-danger" type="submit"
                @click="onclickDeleteElement(element.id)">
                <i class="fas fa-trash"></i>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Swal from 'sweetalert2';
export default {
  props: {
    indexElementRoute: {
      type: String,
      required: true
    },
    updateElementRoute: {
      type: String,
      required: true
    },
    deleteElementRoute: {
      type: String,
      required: true
    },
  },
  data() {
    return {
      elements: [],
      titles: {},
    }
  },
  methods: {
    updateElement(elementId) {

      axios.put(this.updateElementRoute.replace('element_id', elementId), {
        title: this.titles[elementId],
      })
        .then(response => {
          Swal.fire({
            position: "top-end",
            title: "Updated successfully!",
            toast: true,
            icon: "success",
            showConfirmButton: false,
            timer: 3000
          });
        })
        .catch(error => {
          this.errors = error.response.data.errors;
        });
    },
    deleteElement(elementId) {
      Swal.fire({
        title: "Are you sure?",
        text: "You won't be able to revert this!",
        icon: "warning",
        showCancelButton: true,
        confirmButtonColor: "#3085d6",
        cancelButtonColor: "#d33",
        confirmButtonText: "Yes, delete it!"
      }).then(result => {
        if (result.value) {
          this.deleteElementConfirmed(elementId);
        }
      });
    },
    deleteElementConfirmed(elementId) {
      axios.delete(this.deleteElementRoute.replace('element_id', elementId))
        .then(response => {
          Swal.fire({
            position: "top-end",
            title: "Delete successfully!",
            toast: true,
            icon: "success",
            showConfirmButton: false,
            timer: 3000
          });
          this.getElements();
        });
    },
    getElements() {
      axios.get(this.indexElementRoute)
        .then(response => {
          this.elements = response.data;
          this.elements.forEach(element => {
            this.$set(this.titles, element.id, element.title);
          });
        });
    },
    getThumbUrl(element) {
      return element.lowthumb_url ? element.lowthumb_url : element.thumb_url;
    },
    onclickUpdateElement(elementId) {
      this.updateElement(elementId);
    },
    onclickDeleteElement(elementId) {
      this.deleteElement(elementId);
    }
  },
  created() {
    this.getElements();
  },
}
</script>
