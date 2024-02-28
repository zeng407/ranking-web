<template>
  <table class="table table-bordered w-100">
    <thead>
      <tr>
        <th>UID</th>
        <th>Email</th>
        <th>名稱</th>
        <th>建立日</th>
        <th>狀態</th>
        <th>操作</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="user in users.data">
        <td>{{ user.id }}</td>
        <td>{{ user.email }}</td>
        <td>{{ user.name }}</td>
        <td>{{ user.created_at | moment('Y-M-D') }}
          <span class="badge badge-secondary">{{ user.created_at | formNow }}</span>
        </td>
        <td>
          <span v-if="isBanned(user)" class="badge badge-danger">停用</span>
          <span v-else class="badge badge-success">正常</span>
        </td>
        <td>
          <button v-if="isBanned(user)" class="btn btn-success" @click="onclickUnBanUser(user)">恢復帳號</button>
          <button v-else class="btn btn-danger" @click="onclickBanUser(user)">停用帳號</button>
        </td>
      </tr>
    </tbody>
  </table>
</template>

<script>
import Swal from 'sweetalert2';
export default {
  props: {
    banUserRoute: {
      type: String,
      required: true
    },
    unbanUserRoute: {
      type: String,
      required: true
    },
    users: {
      type: Object,
      required: true
    },
  },
  data() {
    return {
    }
  },
  methods: {
    banUser(user) {
      Swal.fire({
        title: "Are you sure?",
        text: "This action will ban the user. Are you sure you want to proceed?",
        icon: "warning",
        showCancelButton: true,
        confirmButtonColor: "#3085d6",
        cancelButtonColor: "#d33",
        confirmButtonText: "Yes, ban user!",
        cancelButtonText: "Cancel"
      }).then(result => {
        if (result.isConfirmed) {
          axios.put(this.banUserRoute.replace('user_id', user.id))
            .then(response => {
              location.reload();
            });
        }
      });
    },
    unbanUser(user) {
      Swal.fire({
        title: "Are you sure?",
        text: "This action will unban the user. Are you sure you want to proceed?",
        icon: "warning",
        showCancelButton: true,
        confirmButtonColor: "#3085d6",
        cancelButtonColor: "#d33",
        confirmButtonText: "Yes, unban user!",
        cancelButtonText: "Cancel"
      }).then(result => {
        if (result.isConfirmed) {
          axios.put(this.unbanUserRoute.replace('user_id', user.id))
            .then(response => {
              location.reload();    
            });
        }
      });
    },
    onclickBanUser(user) {
      this.banUser(user);
    },
    onclickUnBanUser(user) {
      this.unbanUser(user);
    },
    isBanned(user) {
      return user.roles.some(role => role.slug === 'banned');
    }
  },
  created() {

  },
}
</script>
