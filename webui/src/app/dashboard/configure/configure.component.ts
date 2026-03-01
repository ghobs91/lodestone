import { Component, inject, OnInit, signal } from "@angular/core";
import {
  FormControl,
  FormGroup,
  ReactiveFormsModule,
} from "@angular/forms";
import { HttpClient } from "@angular/common/http";
import { catchError, EMPTY, finalize } from "rxjs";
import { AppModule } from "../../app.module";
import { DocumentTitleComponent } from "../../layout/document-title.component";
import { ErrorsService } from "../../errors/errors.service";
import { MatSnackBar } from "@angular/material/snack-bar";
import { COMMA, ENTER } from "@angular/cdk/keycodes";

export interface ClassifierSettings {
  deleteXxx: boolean;
  bannedKeywords: string[];
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

  readonly separatorKeysCodes = [ENTER, COMMA] as const;

  loading = signal(true);
  saving = signal(false);

  form = new FormGroup({
    deleteXxx: new FormControl<boolean>(false, { nonNullable: true }),
    bannedKeywords: new FormControl<string[]>([], { nonNullable: true }),
  });

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
        this.snackBar.open("Settings saved", "Dismiss", { duration: 3000 });
      });
  }
}

