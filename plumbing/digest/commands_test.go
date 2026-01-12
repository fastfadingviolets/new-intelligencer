package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func setupTestPosts(t *testing.T, dir string, count int) {
	posts := make([]Post, count)
	index := make(PostsIndex)
	for i := 0; i < count; i++ {
		rkey := fmt.Sprintf("rkey_%03d", i)
		posts[i] = Post{
			Rkey: rkey,
			URI:  fmt.Sprintf("at://did:plc:test/app.bsky.feed.post/%s", rkey),
			Text: fmt.Sprintf("Test post %d", i),
		}
		index[rkey] = i
	}
	require.NoError(t, SavePosts(filepath.Join(dir, "posts.json"), posts))
	require.NoError(t, SaveIndex(filepath.Join(dir, "posts-index.json"), index))
	require.NoError(t, SaveCategories(filepath.Join(dir, "categories.json"), Categories{}))
}

func setupTestStoryGroups(t *testing.T, dir string, count int) {
	storyGroups := make(StoryGroups)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("sg_%03d", i)
		storyGroups[id] = StoryGroup{
			ID:        id,
			SectionID: "test-section",
			PostRkeys: []string{fmt.Sprintf("rkey_%d", i)},
		}
	}
	require.NoError(t, SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups))
}

func initTestBatchProgress(t *testing.T, dir string) {
	bp := BatchProgress{}
	data, _ := json.MarshalIndent(bp, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "batches.json"), data, 0644))
}

// Test 1: Verify withLock serializes access

func TestWithLock_SerializesAccess(t *testing.T) {
	dir := t.TempDir()
	counterFile := filepath.Join(dir, "counter.txt")
	require.NoError(t, os.WriteFile(counterFile, []byte("0"), 0644))

	var wg sync.WaitGroup
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := withLock(dir, "test.lock", func() error {
				// Read-modify-write (the race-prone pattern)
				data, err := os.ReadFile(counterFile)
				if err != nil {
					return err
				}
				count, err := strconv.Atoi(string(data))
				if err != nil {
					return err
				}
				count++
				return os.WriteFile(counterFile, []byte(strconv.Itoa(count)), 0644)
			})
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// If locking works, counter should equal numGoroutines
	data, err := os.ReadFile(counterFile)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(numGoroutines), string(data))
}

// Test 2: Concurrent story group updates (simulates parallel headline editors)

func TestUpdateStory_ConcurrentUpdates(t *testing.T) {
	dir := t.TempDir()
	setupTestStoryGroups(t, dir, 10)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			storyID := fmt.Sprintf("sg_%03d", n)
			headline := fmt.Sprintf("Headline %d", n)
			priority := n + 1 // priorities 1-10
			err := updateStoryInDir(dir, storyID, headline, priority, "", false, false)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all 10 stories have correct headlines and priorities
	storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
	require.NoError(t, err)
	assert.Len(t, storyGroups, 10)

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("sg_%03d", i)
		story, ok := storyGroups[id]
		require.True(t, ok, "story %s not found", id)
		assert.Equal(t, fmt.Sprintf("Headline %d", i), story.Headline)
		assert.Equal(t, i+1, story.Priority)
	}
}

// Test 3: Concurrent categorization (simulates parallel categorizers)

func TestCategorize_ConcurrentCategorization(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 100)

	var wg sync.WaitGroup
	sections := []string{"tech", "politics", "sports", "music", "arts"}

	// 5 goroutines each categorizing 20 posts to different sections
	for i, section := range sections {
		wg.Add(1)
		go func(s string, start int) {
			defer wg.Done()
			rkeys := []string{}
			for j := 0; j < 20; j++ {
				rkeys = append(rkeys, fmt.Sprintf("rkey_%03d", start+j))
			}
			err := categorizeInDir(dir, s, rkeys, false)
			assert.NoError(t, err)
		}(section, i*20)
	}

	wg.Wait()

	// Verify all 100 posts categorized, no duplicates, no missing
	cats, err := LoadCategories(filepath.Join(dir, "categories.json"))
	require.NoError(t, err)

	total := 0
	for _, cat := range cats {
		total += len(cat.Visible)
	}
	assert.Equal(t, 100, total)
}

// Test 4: Concurrent story group creation (simulates parallel consolidators)

func TestCreateStoryGroup_ConcurrentCreation(t *testing.T) {
	dir := t.TempDir()
	// Initialize empty story groups file
	require.NoError(t, SaveStoryGroups(filepath.Join(dir, "story-groups.json"), StoryGroups{}))

	var wg sync.WaitGroup
	sections := []string{"tech", "politics", "sports", "music"}

	for _, section := range sections {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			// Each consolidator creates 3 story groups in its section
			for i := 0; i < 3; i++ {
				rkeys := []string{fmt.Sprintf("%s_rkey_%d", s, i)}
				err := createStoryGroupInDir(dir, s, rkeys, "", "")
				assert.NoError(t, err)
			}
		}(section)
	}

	wg.Wait()

	storyGroups, err := LoadStoryGroups(filepath.Join(dir, "story-groups.json"))
	require.NoError(t, err)
	assert.Len(t, storyGroups, 12) // 4 sections × 3 stories
}

// Test 5: Concurrent batch marking (simulates parallel agents marking completion)

func TestMarkBatchDone_ConcurrentMarks(t *testing.T) {
	dir := t.TempDir()
	initTestBatchProgress(t, dir)

	var wg sync.WaitGroup

	// 5 goroutines marking categorization batches
	for offset := 0; offset < 500; offset += 100 {
		wg.Add(1)
		go func(off int) {
			defer wg.Done()
			err := markBatchDoneInDir(dir, "categorization", off, 100, "")
			assert.NoError(t, err)
		}(offset)
	}

	wg.Wait()

	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)
	assert.Len(t, bp.Categorization, 5)
}

// Test 6: Concurrent consolidation marking

func TestMarkBatchDone_ConcurrentConsolidation(t *testing.T) {
	dir := t.TempDir()
	initTestBatchProgress(t, dir)

	var wg sync.WaitGroup
	sections := []string{"tech", "politics", "sports", "music", "arts", "books"}

	for _, section := range sections {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			err := markBatchDoneInDir(dir, "consolidation", 0, 0, s)
			assert.NoError(t, err)
		}(section)
	}

	wg.Wait()

	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)
	assert.Len(t, bp.Consolidation, 6)
}

// Test 7: Concurrent headline marking

func TestMarkBatchDone_ConcurrentHeadlines(t *testing.T) {
	dir := t.TempDir()
	initTestBatchProgress(t, dir)

	var wg sync.WaitGroup
	sections := []string{"tech", "politics", "sports", "music", "arts", "books"}

	for _, section := range sections {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			err := markBatchDoneInDir(dir, "headlines", 0, 0, s)
			assert.NoError(t, err)
		}(section)
	}

	wg.Wait()

	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)
	assert.Len(t, bp.Headlines, 6)
}

// Test 8: Idempotent batch marking (same batch marked twice)

func TestMarkBatchDone_Idempotent(t *testing.T) {
	dir := t.TempDir()
	initTestBatchProgress(t, dir)

	var wg sync.WaitGroup

	// Multiple goroutines all trying to mark the same batch
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := markBatchDoneInDir(dir, "categorization", 0, 100, "")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)
	// Should only have 1 entry, not 10
	assert.Len(t, bp.Categorization, 1)
}

// Test 9: isStageComplete for categorization

func TestIsStageComplete_Categorization(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 250) // 3 batches expected (ceil(250/100) = 3)
	initTestBatchProgress(t, dir)

	wd, err := LoadWorkspace(dir)
	require.NoError(t, err)
	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)

	// No batches done yet
	assert.False(t, isStageComplete("categorization", wd, bp))

	// Mark 2 of 3 batches done
	require.NoError(t, markBatchDoneInDir(dir, "categorization", 0, 100, ""))
	require.NoError(t, markBatchDoneInDir(dir, "categorization", 100, 100, ""))
	bp, _ = loadBatchProgress(dir)
	assert.False(t, isStageComplete("categorization", wd, bp))

	// Mark final batch done
	require.NoError(t, markBatchDoneInDir(dir, "categorization", 200, 100, ""))
	bp, _ = loadBatchProgress(dir)
	assert.True(t, isStageComplete("categorization", wd, bp))
}

// Test 10: isStageComplete for consolidation

func TestIsStageComplete_Consolidation(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 10)
	initTestBatchProgress(t, dir)

	// Setup categories with known sections
	cats := Categories{
		"tech":   CategoryData{Visible: []string{"rkey_001"}},
		"sports": CategoryData{Visible: []string{"rkey_002"}},
	}
	require.NoError(t, SaveCategories(filepath.Join(dir, "categories.json"), cats))

	wd, err := LoadWorkspace(dir)
	require.NoError(t, err)
	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)

	// No sections consolidated
	assert.False(t, isStageComplete("consolidation", wd, bp))

	// Mark one section
	require.NoError(t, markBatchDoneInDir(dir, "consolidation", 0, 0, "tech"))
	bp, _ = loadBatchProgress(dir)
	assert.False(t, isStageComplete("consolidation", wd, bp))

	// Mark all sections
	require.NoError(t, markBatchDoneInDir(dir, "consolidation", 0, 0, "sports"))
	bp, _ = loadBatchProgress(dir)
	assert.True(t, isStageComplete("consolidation", wd, bp))
}

// Test 11: isStageComplete for headlines

func TestIsStageComplete_Headlines(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 10)
	initTestBatchProgress(t, dir)

	// Create story groups in two sections
	storyGroups := StoryGroups{
		"story1": StoryGroup{ID: "story1", SectionID: "tech", PostRkeys: []string{"rkey_001"}},
		"story2": StoryGroup{ID: "story2", SectionID: "music", PostRkeys: []string{"rkey_002"}},
	}
	require.NoError(t, SaveStoryGroups(filepath.Join(dir, "story-groups.json"), storyGroups))

	wd, err := LoadWorkspace(dir)
	require.NoError(t, err)
	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)

	// No headlines done
	assert.False(t, isStageComplete("headlines", wd, bp))

	// Mark one section
	require.NoError(t, markBatchDoneInDir(dir, "headlines", 0, 0, "tech"))
	bp, _ = loadBatchProgress(dir)
	assert.False(t, isStageComplete("headlines", wd, bp))

	// Mark all sections
	require.NoError(t, markBatchDoneInDir(dir, "headlines", 0, 0, "music"))
	bp, _ = loadBatchProgress(dir)
	assert.True(t, isStageComplete("headlines", wd, bp))
}

// Test 12: waitForStage timeout

func TestWaitForStage_Timeout(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 100)
	initTestBatchProgress(t, dir)

	// Should timeout (200ms) since no batches are marked done
	err := waitForStage(dir, "categorization", 200*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// Test 13: waitForStage immediate completion

func TestWaitForStage_AlreadyComplete(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 50) // 1 batch
	initTestBatchProgress(t, dir)

	// Pre-mark batch as done
	require.NoError(t, markBatchDoneInDir(dir, "categorization", 0, 100, ""))

	// Should return immediately
	err := waitForStage(dir, "categorization", 1*time.Second, 100*time.Millisecond)
	assert.NoError(t, err)
}

// Test 14: getStageProgress output

func TestGetStageProgress(t *testing.T) {
	dir := t.TempDir()
	setupTestPosts(t, dir, 250) // 3 batches
	initTestBatchProgress(t, dir)

	wd, err := LoadWorkspace(dir)
	require.NoError(t, err)
	bp, err := loadBatchProgress(dir)
	require.NoError(t, err)

	// Initially 0/3
	progress := getStageProgress("categorization", wd, bp)
	assert.Equal(t, "0/3 batches complete", progress)

	// After 2 batches
	markBatchDoneInDir(dir, "categorization", 0, 100, "")
	markBatchDoneInDir(dir, "categorization", 100, 100, "")
	bp, _ = loadBatchProgress(dir)
	progress = getStageProgress("categorization", wd, bp)
	assert.Equal(t, "2/3 batches complete", progress)
}
