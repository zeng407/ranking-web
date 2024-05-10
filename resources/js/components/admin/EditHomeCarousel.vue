<template>
  <div>
    <!-- create carousel on modal -->
    <button class="btn btn-primary mb-2 fa-pull-right" @click="showCreateModal = true">Create Carousel</button>
    
    <create-home-carousel-modal 
      v-if="showCreateModal" 
      :close="() => {showCreateModal = false;loadItems()}"
      :createEndpoint="createEndpoint">
    </create-home-carousel-modal>

    <update-home-carousel-modal 
      v-if="showEditModal" 
      :close="() => {showEditModal = false;loadItems()}"
      :updateEndpoint="updateEndpoint"
      :item="temp_editing_item">
    </update-home-carousel-modal>

    <table class=" table table-bordered">
      <thead>
        <tr>
          <th>#</th>
          <th scope="col">Image</th>
          <th scope="col">Type</th>
          <th scope="col">Title</th>
          <th scope="col">Description</th>
          <th scope="col">Actions</th>
        </tr>
      </thead>
      <draggable v-model="items" @start="drag = true" @end="drag = false" tag="tbody" @change="handleChange">
        <tr v-for="(item, index) in items" :key="item.id">
          <td class=" cursor-pointer">{{ index + 1 }}</td>
          <td>
            <img :src="item.image_url" alt="Image" class="img-thumbnail" style="width: 100px;">
          </td>
          <td>{{ item.type }}</td>
          <td>{{ item.title }}</td>
          <td>{{ item.description }}</td>
          <td>
            <button class="btn btn-primary btn-sm" @click="editItem(item.id)">Edit</button>
            <button class="btn btn-danger btn-sm" @click="deleteItem(item.id)">Delete</button>
          </td>
        </tr>
      </draggable>
    </table>

  </div>
</template>

<script>
import Swal from 'sweetalert2';
import draggable from 'vuedraggable'

export default {
  components: {
    draggable,
  },
  props: {
    indexEndpoint: {
      type: String,
      required: true
    },
    createEndpoint: {
      type: String,
      required: true
    },
    reorderEndpoint: {
      type: String,
      required: true
    },
    deleteEndpoint: {
      type: String,
      required: true
    },
    updateEndpoint: {
      type: String,
      required: true
    },
  },
  data() {
    return {
      drag: false,
      items: [], // your items data
      showCreateModal: false,
      showEditModal: false,
      temp_editing_item: {}
    }
  },
  methods: {
    loadItems() {
      axios.get(this.indexEndpoint)
        .then(response => {
          this.items = response.data;
          console.log(response.data);
        })
        .catch(error => {
          console.log(error.response.data);
          Swal.fire({
            icon: 'error',
            title: 'Error',
            text: 'Unable to load home carousel items',
          });
        });
    },
    editItem(id) {
      this.temp_editing_item = this.items.find(item => item.id === id);
      this.showEditModal = true;
    },
    handleChange() {
      if (this.drag) {
        axios.put(this.reorderEndpoint, {
          items: this.items
        })
          .then(response => {
            console.log(response.data);
            Swal.fire({
              icon: 'success',
              toast: true,
              position: 'top-end',
              text: 'Home carousel items reordered successfully',
              timer: 3000,
            });
          })
          .catch(error => {
            console.log(error.response.data);
            Swal.fire({
              icon: 'error',
              title: 'Error',
              text: 'Unable to update home carousel items',
            });
          });
      }
    },
    deleteItem(id) {
      Swal.fire({
        title: 'Are you sure?',
        text: 'You will not be able to recover this item!',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Yes, delete it!',
        cancelButtonText: 'No, keep it'
      }).then((result) => {
        if (result.value) {
          const route = this.deleteEndpoint.replace('_id', id);
          axios.delete(route)
            .then(response => {
              console.log(response.data);
              Swal.fire({
                icon: 'success',
                toast: true,
                position: 'top-end',
                text: 'Home carousel item deleted successfully',
                timer: 3000,
              });
              this.loadItems();
            })
            .catch(error => {
              console.log(error.response.data);
              Swal.fire({
                icon: 'error',
                title: 'Error',
                text: 'Unable to delete home carousel item',
              });
            });
        }
      });
    },
  },
  mounted() {
    this.loadItems();
  },
}
</script>
