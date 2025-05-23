package cmd

import (
	"context"
	"fmt"

	"github.com/ewhauser/bazel-differ/internal"
	"github.com/ewhauser/bazel-differ/internal/cache"
	"github.com/spf13/cobra"
)

var startingRevision string
var finalRevision string
var query string
var cacheDir string
var cacheDisabled bool
var outputOnEmpty bool

// getTargets represents the get-targets command
var getTargets = &cobra.Command{
	Use:   "get-targets",
	Short: "Collects a set of a targets between the specified commit ranges",
	Long: `Collects a set of a targets between the specified commit ranges. By default, 
the final set of targets is run through a Bazel query "set({{.Targets}})" which can be customized by the -q parameter. 
If you want to query all tests impacted for the given commit range, you can do:

$ bazel-differ get-targets -w path/to/workspace -b $(which bazel) -s START_HASH -f FINAL_HASH -q 'kind(".*_test",set({{.Targets}}))'" -o test_targets.txt
`,
	Run: func(cmd *cobra.Command, args []string) {
		gitClient := internal.NewGitClient(WorkspacePath)
		bazelClient := GetBazelClient()
		cacheManager, err := cache.NewHashCacheManager(!cacheDisabled, cacheDir)
		ExitIfError(err, "error creating hash cache manager")
		targetHasher := internal.NewTargetHashingClient(GetBazelClient(), internal.Filesystem,
			internal.NewRuleProvider())

		startingHashes := getHashes(startingRevision, gitClient, cacheManager, targetHasher)
		endingHashes := getHashes(finalRevision, gitClient, cacheManager, targetHasher)

		targets, err := targetHasher.GetImpactedTargets(startingHashes, endingHashes)
		ExitIfError(err, "error getting impacted targets")

		queriedTargets, err := bazelClient.QueryTarget(query, targets)
		ExitIfError(err, "error querying targets")
		targetNames := targetHasher.GetNames(queriedTargets)

		if Output != "" {
			if len(targetNames) == 0 && !outputOnEmpty {
				return
			}
			internal.WriteTargetsFile(targetNames, Output)
		}

		if Output == "" || Verbose {
			for k := range targetNames {
				fmt.Println(k)
			}
		}
	},
}

func getHashes(revision string, gitClient internal.GitClient, cacheManager cache.HashCacheManager,
	targetHasher internal.TargetHashingClient) map[string]string {
	output, err := gitClient.Checkout(revision)
	ExitIfError(err, fmt.Sprintf("Unable to checkout revision: %s. %s", revision, output))

	var seedfilePaths = make(map[string]bool)
	if SeedFilepaths != "" {
		seedfilePaths = readSeedFile()
	}

	hashes, err := cacheManager.Get(context.Background(), revision)
	ExitIfError(err, fmt.Sprintf("Error retrieving hashes for revision %s from cache", revision))
	if hashes != nil {
		return hashes
	}

	hashes, err = targetHasher.HashAllBazelTargetsAndSourcefiles(seedfilePaths)
	ExitIfError(err, "error hashing files")

	err = cacheManager.Put(context.Background(), revision, hashes)
	ExitIfError(err, "error updating cache")

	return hashes
}

func init() {
	rootCmd.AddCommand(getTargets)
	getTargets.PersistentFlags().StringVarP(&startingRevision, "startingRevision", "s", "",
		"The Git revision to generate the ending hashes")
	getTargets.PersistentFlags().StringVarP(&finalRevision, "finalRevision", "f", "",
		"The Git revision to use to generate the ending hashes")
	getTargets.Flags().StringVarP(&SeedFilepaths, "seed-filepaths", "", "",
		"A text file containing a newline separated list of filepaths, "+
			"each of these filepaths will be read and used as a seed for all targets.")
	getTargets.PersistentFlags().StringVarP(&query, "query", "q", "",
		"The query template to use when querying for changed targets")
	getTargets.PersistentFlags().StringVarP(&Output, "output", "o", "",
		"Filepath to write the impacted Bazel targets to, "+
			"newline separated")
	getTargets.PersistentFlags().BoolVar(&outputOnEmpty, "output-on-empty", true,
		"Sets whether to write a file output when there are no results; useful for skipping targets in CI")
	getTargets.PersistentFlags().StringVar(&cacheDir, "cache-dir", "",
		"Directory to cache hashes associated with commits")
	getTargets.PersistentFlags().BoolVar(&cacheDisabled, "nocache", false,
		"Disables hash caching")
}
