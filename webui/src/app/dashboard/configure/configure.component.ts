import { Component, inject, OnInit, signal } from "@angular/core";
import { FormControl, FormGroup, ReactiveFormsModule } from "@angular/forms";
import { COMMA, ENTER } from "@angular/cdk/keycodes";
import { HttpClient, HttpEventType } from "@angular/common/http";
import { MatSnackBar } from "@angular/material/snack-bar";
import { catchError, EMPTY, finalize } from "rxjs";
import { AppModule } from "../../app.module";
import { DocumentTitleComponent } from "../../layout/document-title.component";
import { ErrorsService } from "../../errors/errors.service";
import { ThemeManager } from "../../themes/theme-manager.service";
import {
  PreferencesService,
  OrderByPreference,
} from "../../preferences/preferences.service";
import { orderByOptions } from "../../torrents/torrents-search.controller";
import { TorrentContentOrderByField } from "../../graphql/generated";

export interface ClassifierSettings {
  deleteXxx: boolean;
  bannedKeywords: string[];
}

export interface TmdbApiKeySettings {
  apiKey: string;
}

interface ImportEventData {
  imported?: number;
  errors?: number;
  done?: boolean;
  error?: string;
}

@Component({
  selector: "app-configure",
  standalone: true,
  imports: [AppModule, DocumentTitleComponent, ReactiveFormsModule],
  templateUrl: "./configure.component.html",
  styleUrl: "./configure.component.scss",
})
export class ConfigureComponent implements OnInit {
  private http = inject(HttpClient);
  private errorsService = inject(ErrorsService);
  private snackBar = inject(MatSnackBar);
  themeManager = inject(ThemeManager);
  preferences = inject(PreferencesService);

  readonly separatorKeysCodes = [ENTER, COMMA] as const;

  loading = signal(true);
  saving = signal(false);
  savingTmdb = signal(false);
  importing = signal(false);
  importProgress = signal("");
  importFile: File | null = null;

  orderByOptions = orderByOptions.filter((o) => o.field !== "relevance");

  form = new FormGroup({
    deleteXxx: new FormControl<boolean>(false, { nonNullable: true }),
    bannedKeywords: new FormControl<string[]>([], { nonNullable: true }),
  });

  tmdbForm = new FormGroup({
    apiKey: new FormControl<string>("", { nonNullable: true }),
  });

  orderByField = new FormControl<TorrentContentOrderByField | "">("", {
    nonNullable: true,
  });
  orderByDesc = new FormControl<boolean>(true, { nonNullable: true });

  get bannedKeywordsValue(): string[] {
    return this.form.controls.bannedKeywords.value ?? [];
  }

  ngOnInit() {
    this.http
      .get<ClassifierSettings>("/api/classifier-settings")
      .pipe(
        catchError((err: Error) => {
          this.errorsService.addError(err.message);
          return EMPTY;
        }),
        finalize(() => this.loading.set(false)),
      )
      .subscribe((settings) => {
        this.form.setValue({
          deleteXxx: settings.deleteXxx ?? false,
          bannedKeywords: settings.bannedKeywords ?? [],
        });
      });

    this.http
      .get<TmdbApiKeySettings>("/api/tmdb-api-key-settings")
      .pipe(catchError(() => EMPTY))
      .subscribe((settings) => {
        this.tmdbForm.setValue({
          apiKey: settings.apiKey ?? "",
        });
      });

    const pref = this.preferences.getDefaultOrderBy();
    if (pref) {
      this.orderByField.setValue(pref.field);
      this.orderByDesc.setValue(pref.descending);
    }
  }

  addKeyword(event: { value: string; chipInput?: { clear: () => void } }) {
    const value = (event.value ?? "").trim();
    if (value) {
      const current = this.bannedKeywordsValue;
      if (!current.includes(value)) {
        this.form.controls.bannedKeywords.setValue([...current, value]);
        this.form.markAsDirty();
      }
    }
    event.chipInput?.clear();
  }

  removeKeyword(keyword: string) {
    const current = this.bannedKeywordsValue;
    this.form.controls.bannedKeywords.setValue(
      current.filter((k) => k !== keyword),
    );
    this.form.markAsDirty();
  }

  save() {
    if (this.saving()) return;
    this.saving.set(true);
    const payload: ClassifierSettings = {
      deleteXxx: this.form.controls.deleteXxx.value,
      bannedKeywords: this.bannedKeywordsValue,
    };
    this.http
      .put<ClassifierSettings>("/api/classifier-settings", payload)
      .pipe(
        catchError((err: Error) => {
          this.errorsService.addError(err.message);
          return EMPTY;
        }),
        finalize(() => this.saving.set(false)),
      )
      .subscribe(() => {
        this.form.markAsPristine();
        this.snackBar.open("Settings saved", "Dismiss", {
          duration: 3000,
        });
      });
  }

  saveTmdbApiKey() {
    if (this.savingTmdb()) return;
    this.savingTmdb.set(true);
    const payload: TmdbApiKeySettings = {
      apiKey: this.tmdbForm.controls.apiKey.value,
    };
    this.http
      .put<TmdbApiKeySettings>("/api/tmdb-api-key-settings", payload)
      .pipe(
        catchError((err: Error) => {
          this.errorsService.addError(err.message);
          return EMPTY;
        }),
        finalize(() => this.savingTmdb.set(false)),
      )
      .subscribe(() => {
        this.tmdbForm.markAsPristine();
        this.snackBar.open("TMDB API key saved", "Dismiss", {
          duration: 3000,
        });
      });
  }

  saveOrderByPreference() {
    const field = this.orderByField.value;
    if (field) {
      const pref: OrderByPreference = {
        field,
        descending: this.orderByDesc.value,
      };
      this.preferences.setDefaultOrderBy(pref);
    } else {
      this.preferences.setDefaultOrderBy(undefined);
    }
    this.snackBar.open("Default sort order saved", "Dismiss", {
      duration: 3000,
    });
  }

  clearOrderByPreference() {
    this.orderByField.setValue("");
    this.orderByDesc.setValue(true);
    this.preferences.setDefaultOrderBy(undefined);
    this.snackBar.open("Default sort order reset", "Dismiss", {
      duration: 3000,
    });
  }

  onImportFileSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      this.importFile = input.files[0];
    }
  }

  startImport() {
    if (!this.importFile || this.importing()) return;
    this.importing.set(true);
    this.importProgress.set("Starting import...");

    const formData = new FormData();
    formData.append("file", this.importFile);

    this.http
      .post("/api/import-sqlite", formData, {
        responseType: "text",
        reportProgress: true,
        observe: "events",
      })
      .pipe(
        catchError((err: Error) => {
          this.errorsService.addError(err.message);
          this.importProgress.set("Import failed: " + err.message);
          return EMPTY;
        }),
        finalize(() => this.importing.set(false)),
      )
      .subscribe((event) => {
        if (
          event.type === HttpEventType.DownloadProgress ||
          event.type === HttpEventType.Response
        ) {
          const body =
            event.type === HttpEventType.Response
              ? (event as { body: string }).body
              : "";
          if (body) {
            const lines = body
              .split("\n")
              .filter((l: string) => l.startsWith("data: "));
            if (lines.length > 0) {
              const lastLine = lines[lines.length - 1].replace("data: ", "");
              try {
                const data = JSON.parse(lastLine) as ImportEventData;
                if (data.done) {
                  this.importProgress.set(
                    `Import complete: ${data.imported} items imported, ${data.errors} errors`,
                  );
                  this.importFile = null;
                } else if (data.error) {
                  this.importProgress.set(`Import error: ${data.error}`);
                } else {
                  this.importProgress.set(
                    `Importing: ${data.imported} items processed, ${data.errors} errors...`,
                  );
                }
              } catch {
                // ignore parse errors
              }
            }
          }
        }
      });
  }
}
