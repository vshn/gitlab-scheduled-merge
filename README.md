# gitlab-scheduled-merge

`gitlab-scheduled-merge` watches GitLab merge requests on a GitLab instance, looking for merge requests with a particular label, and takes care of automatically merging them at specific times based on an in-repo configuration file.

## Getting Started

You'll need a GitLab access token with read-write access to all the repositories on which you want to enable scheduled automerge.

```
./gitlab-scheduled-merge -t [GITLAB_TOKEN] --gitlab-base-url [BASE_URL]
```

Your repositories need to be configured for scheduled automerge by adding a config file to the tree, which defines the time windows during which scheduled MRs can be merged.
By default, this file is called `.merge-schedule.yml`.

`.merge-schedule.yml`:
```
mergeWindows:
- schedule:
    cron: '0 2 * * *' # cron schedule which specifies the start of each merge window
    isoWeek: '@even' # optional, can be @even or @odd to restrict the cron schedule to even/odd week numbers, or an integer to restrict it to one specific week of the year
    location: 'Europe/Zurich' # optional, specify the time zone to interpret the cron schedule
  maxDelay: '1h' # duration for which the merge window remains active
```

The config file is taken from the source branch of the merge request that is to be scheduled.

If multiple schedules are specified, merge requests are merged if at least one of them is active.

Whenever a merge request is labeled with the correct label (by default `scheduled`), the application will find it and merge it if a merge window is currently active, or post a comment indicating when the next merge window takes place.

## License

BSD 3-Clause License

Copyright (c) 2024, VSHN AG

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived from
   this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
