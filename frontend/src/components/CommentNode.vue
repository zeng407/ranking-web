<script setup lang="ts">
import { computed, inject } from 'vue'

import { translate, type MessageKey } from '../i18n'
import {
  avatarInitial,
  commentThreadKey,
  maxCommentDepth,
  relativeTime,
  type CommentThread,
} from './commentThread'

const props = defineProps<{ node: CommentThread }>()

const context = inject(commentThreadKey)
if (!context) throw new Error('CommentNode must be rendered inside a comment section')
const thread = context

const comment = computed(() => props.node.comment)
const replying = computed(() => thread.reply.targetID === comment.value.id)
// The last level answers the level above it, and there is nowhere further to go.
const canReply = computed(() => !comment.value.deleted && comment.value.depth < maxCommentDepth)

function t(key: MessageKey): string {
  return translate(thread.locale, key)
}
</script>

<template>
  <li class="comment-card" :class="{ 'is-reply': comment.depth > 1, 'is-deleted': comment.deleted }">
    <div class="comment-avatar" aria-hidden="true">
      <template v-if="comment.deleted">
        <span class="comment-avatar-empty"></span>
      </template>
      <img v-else-if="comment.avatar_url" :src="comment.avatar_url" alt="" loading="lazy">
      <span v-else>{{ avatarInitial(comment.nickname) }}</span>
    </div>
    <article>
      <header>
        <div>
          <span v-if="comment.floor !== null" class="comment-floor">{{ comment.floor }}F</span>
          <strong v-if="!comment.deleted" class="comment-author">{{ comment.nickname }}</strong>
          <time :datetime="comment.created_at" :title="comment.created_at">
            {{ relativeTime(comment.created_at, thread.locale) }}
          </time>
        </div>
        <div v-if="!comment.deleted" class="comment-card-actions">
          <button
            v-if="comment.can_delete"
            class="comment-report-button comment-delete-button"
            type="button"
            :aria-label="t('commentDelete')"
            :title="t('commentDelete')"
            @click="thread.confirmDelete(comment)"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/></svg>
          </button>
          <button
            class="comment-report-button"
            type="button"
            :aria-label="t('commentReport')"
            :title="t('commentReport')"
            @click="thread.openReport(comment)"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></svg>
          </button>
        </div>
      </header>

      <p v-if="comment.deleted" class="comment-deleted">{{ t('commentDeleted') }}</p>
      <template v-else>
        <div v-if="comment.champions.length" class="comment-champions">
          <span v-for="champion in comment.champions" :key="champion" class="comment-champion">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 4h8v4a4 4 0 0 1-8 0V4ZM6 6H3v1a4 4 0 0 0 5 4M18 6h3v1a4 4 0 0 1-5 4M12 12v4M8 20h8M9 16h6"/></svg>
            {{ champion }}
          </span>
        </div>
        <p class="comment-content">
          <span v-if="node.addressee" class="comment-mention">@{{ node.addressee }}</span>{{ comment.content }}
        </p>
        <button v-if="canReply" class="comment-reply-button" type="button" @click="thread.openReply(comment)">
          {{ t('commentReply') }}
        </button>
      </template>

      <form v-if="replying" class="comment-reply-form" @submit.prevent="thread.submitReply()">
        <label class="sr-only" :for="`comment-reply-${comment.id}`">{{ t('commentReply') }}</label>
        <textarea
          :id="`comment-reply-${comment.id}`"
          v-model="thread.reply.input"
          class="comment-input"
          rows="2"
          maxlength="200"
          :placeholder="t('commentReplyPlaceholder')"
        ></textarea>
        <div class="comment-reply-actions">
          <button class="button button-quiet" type="button" :disabled="thread.reply.submitting" @click="thread.cancelReply()">
            {{ t('commentCancel') }}
          </button>
          <button
            class="button button-primary comment-reply-submit"
            type="button"
            :disabled="!thread.reply.input.trim() || thread.reply.submitting"
            @click="thread.submitReply()"
          >
            {{ thread.reply.submitting ? t('commentSubmitting') : t('commentReplySubmit') }}
          </button>
        </div>
        <p v-if="thread.reply.error" class="comment-error" role="alert">{{ thread.reply.error }}</p>
      </form>

      <ol v-if="node.replies.length" class="comment-replies">
        <CommentNode v-for="reply in node.replies" :key="reply.comment.id" :node="reply" />
      </ol>
    </article>
  </li>
</template>
